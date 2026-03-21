/*
The fs package provides a 9P filesystem that exposes the in-memory data set containing
denote metadata for all files in the denote directory. The filesystem is organized as
follows:

	denote/                 (directory)  Root filesystem
		ctl					(write-only) Control file (commands: filter <query>, cd <path>)
		event				(read-only)  Global event bus (listen & react to rename, update, and delete events)
		index				(read-only)  Metadata index (respects active filter)
		new					(write-only) Create new note (write "'title' tag1,tag2")
		n/					(directory)  Notes directory
			<identifier>/   (directory)  Unique denote file identifier
				backlinks	(read-only)  Notes that link to this note (same format as index)
				ctl			(write-only) Control file (publish rename, update, and delete events)
				keywords	(read-write) File tags
				path		(read-write) Path on underlying filesystem
				title		(read-write) Denote document title

Notes:
- Messages written to denote/ctl affect global state (e.g., filtering, directory switching)
- Filter command syntax: filter [field:]pattern where field is date|title|tag
- Multiple filters can be specified: filter tag:journal !tag:draft
- Titles with spaces must be quoted: filter title:"My Title"
- Writing 'filter' with no arguments clears the active filter
- Index respects the current filter - returns only matching notes
- Cd command syntax: cd /path/to/directory
- After cd, run Get to reload notes from the new directory
- Messages written to denote/n/<identifier>/ctl may generate events that are broadcast over the global event bus
- Writing to title or keywords triggers an update ('u') event and a rename ('r') event
- Write 'd' to denote/n/<identifier>/ctl to delete a denote file
- Write "'document title' tag1,tag2,(...)" to denote/new to create a new denote file
*/
package fs

import (
	"denotesrv/internal/disk"
	"denotesrv/pkg/encoding/frontmatter"
	"denotesrv/pkg/encoding/results"
	"denotesrv/pkg/metadata"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"9fans.net/go/plan9"
	"9fans.net/go/plan9/client"
)

// File types in our filesystem
const (
	QTDir  = plan9.QTDIR
	QTFile = plan9.QTFILE
)

// Qid paths - we'll use a simple scheme:
// 0: root
// 1-1000000: note directories (qid = 1 + note_index)
// 1000001+: files (qid = 1000001 + note_index*100 + file_type)
const (
	qidRoot     = 0
	qidNoteBase = 1
	qidFileBase = 1000001
	qidIndex    = 999999
	qidNew      = 999997
	qidNDir     = 999996
	qidCtl      = 999995
	qidDir      = 999994
)

var fileNames = []string{"path", "title", "keywords", "signature", "ctl", "backlinks", "body"}

// Callbacks for note operations
type Callbacks struct {
	OnNew    func(identifier string) error
	OnUpdate func(identifier string) error
	OnRename func(identifier string) error
	OnDelete func(identifier string) error
}

type server struct {
	notes         metadata.Results
	denoteDir     string
	mu            sync.RWMutex
	callbacks     Callbacks
	filterQuery   string
	filteredNotes []*metadata.Metadata
}

type connState struct {
	fids map[uint32]*fid
	mu   sync.RWMutex
}

type fid struct {
	qid         plan9.Qid
	path        string
	offset      int64
	mode        uint8
	writeBuf    []byte // accumulates Twrite chunks, dispatched on Tclunk
}

var srv *server

// GetDenoteDir returns the current denote directory.
// Returns empty string if server not yet initialized.
func GetDenoteDir() string {
	if srv == nil {
		return ""
	}
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	return srv.denoteDir
}

// findNote finds a note by identifier in the in-memory collection
func (s *server) findNote(identifier string) (*metadata.Metadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, n := range s.notes {
		if n.Identifier == identifier {
			return n, nil
		}
	}
	return nil, fmt.Errorf("note not found: %s", identifier)
}

// UpdateMetadataFromDisk updates note metadata directly from disk without
// triggering callbacks or 9P protocol overhead. Used by GetAll() for bulk sync.
func UpdateMetadataFromDisk(identifier, title, keywords, signature string) error {
	if srv == nil {
		return fmt.Errorf("server not running")
	}

	note, err := srv.findNote(identifier)
	if err != nil {
		return err
	}

	// Parse keywords into tags
	var tags []string
	if keywords != "" {
		tags = strings.Split(keywords, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	// Update fields directly (note is a pointer, so this modifies the original)
	note.Title = title
	note.Tags = tags
	note.Signature = signature

	return nil
}

// Getdir returns the denote directory
// StartServer starts the 9P fileserver in the background with pre-loaded metadata.
// initialData should contain all notes to be served - typically loaded by sync.LoadAll().
// Server is the 9P server for denote notes
type Server struct {
	*server
}

// NewServer creates a new denote 9P server
func NewServer(denoteDir string, callbacks Callbacks) (*Server, error) {
	// Load notes from disk
	notes, err := disk.LoadAll(denoteDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load notes: %w", err)
	}

	s := &server{
		notes:     notes,
		denoteDir: denoteDir,
		callbacks: callbacks,
	}
	return &Server{s}, nil
}

// Serve handles a single 9P connection
func (s *Server) Serve(conn net.Conn) {
	s.server.serve(conn)
}

func StartServer(initialData metadata.Results, denoteDir string, callbacks Callbacks) error {
	if srv != nil {
		return fmt.Errorf("server already running")
	}

	srv = &server{
		notes:     initialData,
		denoteDir: denoteDir,
		callbacks: callbacks,
	}

	// Get namespace and create Unix socket path
	ns := client.Namespace()
	if ns == "" {
		return fmt.Errorf("failed to get namespace")
	}

	sockPath := ns + "/denote"

	// Try to create Unix domain socket listener
	// Use stale socket detection like acme (see /usr/local/plan9/src/lib9/announce.c)
	var listener net.Listener
	var err error
	for attempts := 0; attempts < 2; attempts++ {
		listener, err = net.Listen("unix", sockPath)
		if err == nil {
			break // Successfully bound to socket
		}

		// Check if error is "address already in use"
		if errors.Is(err, syscall.EADDRINUSE) {
			// Try to connect to see if socket is live or stale
			conn, dialErr := net.Dial("unix", sockPath)
			if dialErr == nil {
				// Connection succeeded - another instance is running
				conn.Close()
				return fmt.Errorf("Denote already running in this namespace")
			}
			// Connection failed - socket is stale, remove it and retry
			os.Remove(sockPath)
			continue
		}

		// Some other error
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	if err != nil {
		return fmt.Errorf("failed to listen on socket after cleanup: %w", err)
	}

	// Start accepting connections in background
	go srv.acceptLoop(listener)

	return nil
}

func (s *server) acceptLoop(listener net.Listener) {
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "denote fs: accept error: %v\n", err)
			return
		}

		go s.serve(conn)
	}
}

func (s *server) serve(conn net.Conn) {
	defer conn.Close()

	cs := &connState{
		fids: make(map[uint32]*fid),
	}

	for {
		fc, err := plan9.ReadFcall(conn)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "denote fs: read error: %v\n", err)
			}
			return
		}

		rfc := s.handle(cs, fc)
		if err := plan9.WriteFcall(conn, rfc); err != nil {
			fmt.Fprintf(os.Stderr, "denote fs: write error: %v\n", err)
			return
		}
	}
}

func (s *server) handle(cs *connState, fc *plan9.Fcall) *plan9.Fcall {
	switch fc.Type {
	case plan9.Tversion:
		return s.version(fc)
	case plan9.Tauth:
		return errorFcall(fc, "denote: authentication not required")
	case plan9.Tattach:
		return s.attach(cs, fc)
	case plan9.Twalk:
		return s.walk(cs, fc)
	case plan9.Topen:
		return s.open(cs, fc)
	case plan9.Tread:
		return s.read(cs, fc)
	case plan9.Twrite:
		return s.write(cs, fc)
	case plan9.Tstat:
		return s.stat(cs, fc)
	case plan9.Tclunk:
		return s.clunk(cs, fc)
	default:
		return errorFcall(fc, "operation not supported")
	}
}

func (s *server) version(fc *plan9.Fcall) *plan9.Fcall {
	msize := min(fc.Msize, 8192)
	return &plan9.Fcall{
		Type:    plan9.Rversion,
		Tag:     fc.Tag,
		Msize:   msize,
		Version: "9P2000",
	}
}

func (s *server) attach(cs *connState, fc *plan9.Fcall) *plan9.Fcall {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	qid := plan9.Qid{
		Type: QTDir,
		Path: qidRoot,
	}

	cs.fids[fc.Fid] = &fid{
		qid:  qid,
		path: "/",
	}

	return &plan9.Fcall{
		Type: plan9.Rattach,
		Tag:  fc.Tag,
		Qid:  qid,
	}
}

func (s *server) walk(cs *connState, fc *plan9.Fcall) *plan9.Fcall {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	f, ok := cs.fids[fc.Fid]
	if !ok {
		return errorFcall(fc, "fid not found")
	}

	// If no wnames, this is a clone operation
	if len(fc.Wname) == 0 {
		newFid := &fid{
			qid:  f.qid,
			path: f.path,
		}
		cs.fids[fc.Newfid] = newFid
		return &plan9.Fcall{
			Type: plan9.Rwalk,
			Tag:  fc.Tag,
			Wqid: []plan9.Qid{},
		}
	}

	// Walk the path
	path := f.path
	qids := []plan9.Qid{}

	for _, name := range fc.Wname {
		if path == "/" {
			// Walking from root
			found := false
			if name == "index" {
				qid := plan9.Qid{
					Type: QTFile,
					Path: uint64(qidIndex),
				}
				qids = append(qids, qid)
				path = "/index"
				found = true
			} else if name == "new" {
				qid := plan9.Qid{
					Type: QTFile,
					Path: uint64(qidNew),
				}
				qids = append(qids, qid)
				path = "/new"
				found = true
			} else if name == "ctl" {
				qid := plan9.Qid{
					Type: QTFile,
					Path: uint64(qidCtl),
				}
				qids = append(qids, qid)
				path = "/ctl"
				found = true
			} else if name == "dir" {
				qid := plan9.Qid{
					Type: QTFile,
					Path: uint64(qidDir),
				}
				qids = append(qids, qid)
				path = "/dir"
				found = true
			} else if name == "n" {
				qid := plan9.Qid{
					Type: QTDir,
					Path: uint64(qidNDir),
				}
				qids = append(qids, qid)
				path = "/n"
				found = true
			}
			if !found {
				return errorFcall(fc, "file not found")
			}
		} else if path == "/n" {
			// Walking from /n - should be a note identifier
			found := false
			for i, note := range s.notes {
				if note.Identifier == name {
					qid := plan9.Qid{
						Type: QTDir,
						Path: uint64(qidNoteBase + i),
					}
					qids = append(qids, qid)
					path = "/n/" + name
					found = true
					break
				}
			}
			if !found {
				return errorFcall(fc, "file not found")
			}
		} else {
			// Walking from note dir - should be a file
			found := false
			for i, fname := range fileNames {
				if fname == name {
					noteIdx := s.pathToNoteIndex(path)
					qid := plan9.Qid{
						Type: QTFile,
						Path: uint64(qidFileBase + noteIdx*100 + i),
					}
					qids = append(qids, qid)
					path = path + "/" + name
					found = true
					break
				}
			}
			if !found {
				return errorFcall(fc, "file not found")
			}
		}
	}

	newFid := &fid{
		qid:  qids[len(qids)-1],
		path: path,
	}
	cs.fids[fc.Newfid] = newFid

	return &plan9.Fcall{
		Type: plan9.Rwalk,
		Tag:  fc.Tag,
		Wqid: qids,
	}
}

func (s *server) open(cs *connState, fc *plan9.Fcall) *plan9.Fcall {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	f, ok := cs.fids[fc.Fid]
	if !ok {
		return errorFcall(fc, "fid not found")
	}

	f.mode = fc.Mode

	return &plan9.Fcall{
		Type: plan9.Ropen,
		Tag:  fc.Tag,
		Qid:  f.qid,
	}
}

func (s *server) read(cs *connState, fc *plan9.Fcall) *plan9.Fcall {
	cs.mu.Lock()

	f, ok := cs.fids[fc.Fid]
	if !ok {
		cs.mu.Unlock()
		return errorFcall(fc, "fid not found")
	}

	defer cs.mu.Unlock()

	var data []byte

	if f.qid.Type&QTDir != 0 {
		// Reading a directory
		if fc.Offset == 0 {
			f.offset = 0
		}
		data = s.readDir(f.path, int64(fc.Offset), fc.Count)
	} else {
		// Reading a file
		content := s.getFileContent(f.path)
		offset := int64(fc.Offset)
		if offset >= int64(len(content)) {
			data = []byte{}
		} else {
			end := min(offset+int64(fc.Count), int64(len(content)))
			data = []byte(content[offset:end])
		}
	}

	return &plan9.Fcall{
		Type:  plan9.Rread,
		Tag:   fc.Tag,
		Count: uint32(len(data)),
		Data:  data,
	}
}

func (s *server) write(cs *connState, fc *plan9.Fcall) *plan9.Fcall {
	cs.mu.Lock()
	f, ok := cs.fids[fc.Fid]
	if !ok {
		cs.mu.Unlock()
		return errorFcall(fc, "fid not found")
	}

	// Check if opened for writing
	if f.mode&plan9.OWRITE == 0 && f.mode&plan9.ORDWR == 0 {
		cs.mu.Unlock()
		return errorFcall(fc, "file not open for writing")
	}

	// Accumulate data into the per-fid write buffer at the given offset.
	// The 9P client splits writes larger than msize into multiple Twrite messages
	// with increasing offsets; we reassemble here and dispatch on Tclunk.
	end := int(fc.Offset) + len(fc.Data)
	if end > len(f.writeBuf) {
		grown := make([]byte, end)
		copy(grown, f.writeBuf)
		f.writeBuf = grown
	}
	copy(f.writeBuf[fc.Offset:], fc.Data)
	cs.mu.Unlock()

	return &plan9.Fcall{Type: plan9.Rwrite, Tag: fc.Tag, Count: uint32(len(fc.Data))}
}

// dispatchWrite processes the fully-assembled write payload for a given path.
// Called from Tclunk after all Twrite chunks have been accumulated.
func (s *server) dispatchWrite(f *fid, tag uint16) *plan9.Fcall {
	fc := &plan9.Fcall{Tag: tag, Data: f.writeBuf}
	input := strings.TrimSpace(string(f.writeBuf))

	// Handle writes to /ctl
	if f.path == "/ctl" {
		return s.handleCtlCommand(fc)
	}

	// Handle writes to /new
	if f.path == "/new" {
		// Parse: 'Title' ==signature tag1,tag2,...
		if !strings.HasPrefix(input, "'") {
			return errorFcall(fc, "title must be single-quoted")
		}

		closeQuote := strings.Index(input[1:], "'")
		if closeQuote == -1 {
			return errorFcall(fc, "title must be single-quoted (missing closing quote)")
		}

		title := input[1 : closeQuote+1]
		if title == "" {
			return errorFcall(fc, "title cannot be empty")
		}

		remainder := strings.TrimSpace(input[closeQuote+2:])
		var signature string
		var tags []string

		if remainder != "" {
			if strings.HasPrefix(remainder, "==") {
				spaceIdx := strings.Index(remainder, " ")
				if spaceIdx == -1 {
					signature = remainder[2:]
				} else {
					signature = remainder[2:spaceIdx]
					remainder = strings.TrimSpace(remainder[spaceIdx+1:])
				}
			}

			if remainder != "" && !strings.HasPrefix(remainder, "==") {
				tagPattern := regexp.MustCompile(`^([\p{Ll}\p{Nd}]+,)*[\p{Ll}\p{Nd}]+$`)
				if !tagPattern.MatchString(remainder) {
					return errorFcall(fc, "tags must be comma-delimited lowercase unicode words (no spaces)")
				}
				tags = strings.Split(remainder, ",")
			}
		}

		identifier := time.Now().Format("20060102T150405")
		meta := &metadata.Metadata{
			Identifier: identifier,
			Signature:  signature,
			Title:      title,
			Tags:       tags,
			Path:       "",
		}

		s.mu.Lock()
		s.notes = append(s.notes, meta)
		s.mu.Unlock()

		if s.callbacks.OnNew != nil {
			go s.callbacks.OnNew(identifier)
		}

		return &plan9.Fcall{Type: plan9.Rwrite, Tag: fc.Tag, Count: uint32(len(f.writeBuf))}
	}

	// Extract note identifier and field name from path
	parts := strings.Split(strings.Trim(f.path, "/"), "/")
	if len(parts) != 3 || parts[0] != "n" {
		return errorFcall(fc, "invalid path")
	}

	noteID := parts[1]
	fieldName := parts[2]

	note, err := s.findNote(noteID)
	if err != nil {
		return errorFcall(fc, err.Error())
	}

	switch fieldName {
	case "path":
		note.Path = input
	case "title":
		note.Title = input
		if s.callbacks.OnUpdate != nil {
			s.callbacks.OnUpdate(noteID)
		}
		if s.callbacks.OnRename != nil {
			s.callbacks.OnRename(noteID)
		}
		if err := s.renameNote(note); err != nil {
			return errorFcall(fc, err.Error())
		}
	case "keywords":
		if input == "" {
			note.Tags = []string{}
		} else {
			tags := strings.Split(input, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
			note.Tags = tags
		}
		if s.callbacks.OnUpdate != nil {
			s.callbacks.OnUpdate(noteID)
		}
		if s.callbacks.OnRename != nil {
			s.callbacks.OnRename(noteID)
		}
		if err := s.renameNote(note); err != nil {
			return errorFcall(fc, err.Error())
		}
	case "signature":
		note.Signature = input
		if s.callbacks.OnUpdate != nil {
			s.callbacks.OnUpdate(noteID)
		}
		if s.callbacks.OnRename != nil {
			s.callbacks.OnRename(noteID)
		}
		if err := s.renameNote(note); err != nil {
			return errorFcall(fc, err.Error())
		}
	case "ctl":
		switch input {
		case "d":
			if s.callbacks.OnDelete != nil {
				s.callbacks.OnDelete(noteID)
			}
			s.mu.Lock()
			for i, n := range s.notes {
				if n.Identifier == noteID {
					s.notes = append(s.notes[:i], s.notes[i+1:]...)
					break
				}
			}
			for i, n := range s.filteredNotes {
				if n.Identifier == noteID {
					s.filteredNotes = append(s.filteredNotes[:i], s.filteredNotes[i+1:]...)
					break
				}
			}
			s.mu.Unlock()
		case "r":
			if s.callbacks.OnRename != nil {
				s.callbacks.OnRename(noteID)
			}
		}
	case "body":
		if err := s.writeBody(note.Path, input, note); err != nil {
			return errorFcall(fc, err.Error())
		}
	default:
		return errorFcall(fc, "field is read-only")
	}

	return &plan9.Fcall{Type: plan9.Rwrite, Tag: fc.Tag, Count: uint32(len(f.writeBuf))}
}

func (s *server) stat(cs *connState, fc *plan9.Fcall) *plan9.Fcall {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	f, ok := cs.fids[fc.Fid]
	if !ok {
		return errorFcall(fc, "fid not found")
	}

	dir := s.pathToDir(f.path, f.qid)
	stat, err := dir.Bytes()
	if err != nil {
		return errorFcall(fc, err.Error())
	}

	return &plan9.Fcall{
		Type: plan9.Rstat,
		Tag:  fc.Tag,
		Stat: stat,
	}
}

func (s *server) clunk(cs *connState, fc *plan9.Fcall) *plan9.Fcall {
	cs.mu.Lock()
	f, ok := cs.fids[fc.Fid]
	if ok && len(f.writeBuf) > 0 {
		// Dispatch accumulated write before clunking
		cs.mu.Unlock()
		if resp := s.dispatchWrite(f, fc.Tag); resp.Type == plan9.Rerror {
			return resp
		}
		cs.mu.Lock()
	}
	delete(cs.fids, fc.Fid)
	cs.mu.Unlock()

	return &plan9.Fcall{
		Type: plan9.Rclunk,
		Tag:  fc.Tag,
	}
}

func (s *server) readDir(path string, offset int64, count uint32) []byte {
	var dirs []plan9.Dir

	if path == "/" {
		// add index node
		dirs = append(dirs, plan9.Dir{
			Qid: plan9.Qid{
				Type: QTFile,
				Path: uint64(qidIndex),
			},
			Mode:   0444,
			Name:   "index",
			Uid:    "denote",
			Gid:    "denote",
			Muid:   "denote",
			Length: 0,
		})
		// add new node
		dirs = append(dirs, plan9.Dir{
			Qid: plan9.Qid{
				Type: QTFile,
				Path: uint64(qidNew),
			},
			Mode:   0222,
			Name:   "new",
			Uid:    "denote",
			Gid:    "denote",
			Muid:   "denote",
			Length: 0,
		})
		// add ctl node
		dirs = append(dirs, plan9.Dir{
			Qid: plan9.Qid{
				Type: QTFile,
				Path: uint64(qidCtl),
			},
			Mode:   0200,
			Name:   "ctl",
			Uid:    "denote",
			Gid:    "denote",
			Muid:   "denote",
			Length: 0,
		})
		// add dir node
		dirs = append(dirs, plan9.Dir{
			Qid: plan9.Qid{
				Type: QTFile,
				Path: uint64(qidDir),
			},
			Mode:   0444,
			Name:   "dir",
			Uid:    "denote",
			Gid:    "denote",
			Muid:   "denote",
			Length: uint64(len(s.denoteDir)),
		})
		// add n directory
		dirs = append(dirs, plan9.Dir{
			Qid: plan9.Qid{
				Type: QTDir,
				Path: uint64(qidNDir),
			},
			Mode:   plan9.DMDIR | 0555,
			Name:   "n",
			Uid:    "denote",
			Gid:    "denote",
			Muid:   "denote",
			Length: 0,
		})
	} else if path == "/index" {

	} else if path == "/n" {
		// List all note IDs
		for i, note := range s.notes {
			dirs = append(dirs, plan9.Dir{
				Qid: plan9.Qid{
					Type: QTDir,
					Path: uint64(qidNoteBase + 1 + i),
				},
				Mode:   plan9.DMDIR | 0555,
				Name:   note.Identifier,
				Uid:    "denote",
				Gid:    "denote",
				Muid:   "denote",
				Length: 0,
			})
		}
	} else {
		// List files in a note directory
		noteIdx := s.pathToNoteIndex(path)
		for i, fname := range fileNames {
			content := s.getFileContent(path + "/" + fname)
			mode := uint32(0444) // read-only by default
			if fname == "title" || fname == "keywords" || fname == "path" || fname == "body" {
				mode = 0644 // writable
			} else if fname == "ctl" {
				mode = 0200 // write-only
			}
			dirs = append(dirs, plan9.Dir{
				Qid: plan9.Qid{
					Type: QTFile,
					Path: uint64(qidFileBase + noteIdx*100 + i),
				},
				Mode:   plan9.Perm(mode),
				Name:   fname,
				Uid:    "denote",
				Gid:    "denote",
				Muid:   "denote",
				Length: uint64(len(content)),
			})
		}
	}

	// Serialize all directory entries to bytes first
	var allData []byte
	for _, dir := range dirs {
		stat, _ := dir.Bytes()
		allData = append(allData, stat...)
	}

	// Return only complete entries starting at offset.
	// 9P requires that directory reads never split an entry across reads.
	if offset >= int64(len(allData)) {
		return []byte{}
	}

	remaining := allData[offset:]
	var result []byte
	for len(remaining) >= 2 {
		entrySize := int(remaining[0]) | int(remaining[1])<<8
		totalSize := entrySize + 2
		if len(remaining) < totalSize {
			break
		}
		if len(result)+totalSize > int(count) {
			break
		}
		result = append(result, remaining[:totalSize]...)
		remaining = remaining[totalSize:]
	}
	return result
}

func (s *server) pathToDir(path string, qid plan9.Qid) plan9.Dir {
	name := path
	if path == "/" {
		name = "."
	} else if strings.Contains(path, "/") {
		parts := strings.Split(path, "/")
		name = parts[len(parts)-1]
	}

	mode := uint32(0444)
	length := uint64(0)
	content := ""

	if qid.Type&QTDir != 0 {
		mode = plan9.DMDIR | 0555
	} else {
		content = s.getFileContent(path)
		length = uint64(len(content))
	}

	return plan9.Dir{
		Qid:    qid,
		Mode:   plan9.Perm(mode),
		Name:   name,
		Uid:    "denote",
		Gid:    "denote",
		Muid:   "denote",
		Length: length,
	}
}

func (s *server) pathToNoteIndex(path string) int {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] != "n" {
		return -1
	}

	noteID := parts[1]
	for i, note := range s.notes {
		if note.Identifier == noteID {
			return i
		}
	}
	return -1
}

func (s *server) getIndexContent() string {
	if s.filterQuery == "" {
		// No filter: return all notes
		return string(results.Marshal(s.notes))
	}

	// Return filtered notes
	var filtered metadata.Results
	for _, note := range s.filteredNotes {
		filtered = append(filtered, note)
	}
	return string(results.Marshal(filtered))
}

func (s *server) getBacklinks(targetID string) string {
	s.mu.RLock()
	notes := s.notes
	denoteDir := s.denoteDir
	s.mu.RUnlock()

	res := disk.FindBacklinks(targetID, denoteDir, notes)
	return string(results.Marshal(res))
}

func (s *server) getFileContent(path string) string {
	if path == "/index" {
		return s.getIndexContent()
	}

	if path == "/dir" {
		return s.denoteDir
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "n" {
		return ""
	}

	noteID := parts[1]
	fileName := parts[2]

	note, err := s.findNote(noteID)
	if err != nil {
		return ""
	}

	switch fileName {
	case "path":
		return note.Path
	case "title":
		return note.Title
	case "keywords":
		return strings.Join(note.Tags, ",")
	case "signature":
		return note.Signature
	case "backlinks":
		return s.getBacklinks(noteID)
	case "body":
		return s.readBody(note.Path)
	}
	return ""
}

// readBody reads the file content at path
func (s *server) readBody(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// renameNote renames the physical file to match current note metadata and updates note.Path.
// It is a no-op when the filename is already correct.
func (s *server) renameNote(note *metadata.Metadata) error {
	if note.Path == "" {
		return nil
	}
	ext := filepath.Ext(note.Path)
	fm := metadata.NewFrontMatter(note.Title, note.Signature, note.Tags, note.Identifier)
	newName := metadata.BuildFilename(fm, ext)
	newPath := filepath.Join(filepath.Dir(note.Path), newName)
	if newPath == note.Path {
		return nil
	}
	if err := os.Rename(note.Path, newPath); err != nil {
		return err
	}
	note.Path = newPath
	return nil
}

// writeBody writes content to file and syncs frontmatter to metadata
func (s *server) writeBody(path, body string, note *metadata.Metadata) error {
	if path == "" {
		return fmt.Errorf("no path")
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return err
	}
	// Parse frontmatter and sync to metadata
	ext := filepath.Ext(path)
	fm, _, _ := frontmatter.Unmarshal([]byte(body), ext)
	if fm != nil && fm.Title != "" {
		note.Title = fm.Title
		note.Tags = fm.Tags
		note.Signature = fm.Signature
	}
	return s.renameNote(note)
}

func errorFcall(fc *plan9.Fcall, msg string) *plan9.Fcall {
	return &plan9.Fcall{
		Type:  plan9.Rerror,
		Tag:   fc.Tag,
		Ename: msg,
	}
}

// handleCtlCommand processes commands written to /ctl
func (s *server) handleCtlCommand(fc *plan9.Fcall) *plan9.Fcall {
	command := strings.TrimSpace(string(fc.Data))

	// Parse command - must start with a known command word
	if strings.HasPrefix(command, "filter") {
		return s.handleFilterCommand(fc, command)
	}

	if strings.HasPrefix(command, "cd ") {
		return s.handleCdCommand(fc, command)
	}

	if command == "refresh" {
		return s.handleRefreshCommand(fc)
	}

	return errorFcall(fc, fmt.Sprintf("unknown ctl command: %s", command))
}

// handleRefreshCommand reloads metadata from disk for all notes
func (s *server) handleRefreshCommand(fc *plan9.Fcall) *plan9.Fcall {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, note := range s.notes {
		if note.Path == "" {
			continue
		}
		data, err := os.ReadFile(note.Path)
		if err != nil {
			continue
		}
		ext := filepath.Ext(note.Path)
		fm, _, _ := frontmatter.Unmarshal(data, ext)
		if fm != nil && fm.Title != "" {
			note.Title = fm.Title
			note.Tags = fm.Tags
			note.Signature = fm.Signature
		}
	}

	return &plan9.Fcall{Type: plan9.Rwrite, Tag: fc.Tag, Count: uint32(len(fc.Data))}
}

// handleFilterCommand processes the 'filter' ctl command
func (s *server) handleFilterCommand(fc *plan9.Fcall, command string) *plan9.Fcall {
	// Extract filter query after "filter" keyword
	query := strings.TrimSpace(strings.TrimPrefix(command, "filter"))

	s.mu.Lock()
	defer s.mu.Unlock()

	if query == "" {
		// Clear filter: filter with no arguments
		s.filterQuery = ""
		s.filteredNotes = nil

		return &plan9.Fcall{
			Type:  plan9.Rwrite,
			Tag:   fc.Tag,
			Count: uint32(len(fc.Data)),
		}
	}

	// Parse filter query into Filter objects
	filters, err := parseFilterQuery(query)
	if err != nil {
		return errorFcall(fc, fmt.Sprintf("invalid filter: %v", err))
	}

	// Apply filters to notes
	var filtered []*metadata.Metadata
	for _, note := range s.notes {
		match := true
		for _, filt := range filters {
			if !filt.IsMatch(note) {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, note)
		}
	}

	// Store filter state
	s.filterQuery = query
	s.filteredNotes = filtered

	return &plan9.Fcall{
		Type:  plan9.Rwrite,
		Tag:   fc.Tag,
		Count: uint32(len(fc.Data)),
	}
}

// handleCdCommand processes the 'cd' ctl command to change denote directory
func (s *server) handleCdCommand(fc *plan9.Fcall, command string) *plan9.Fcall {
	// Extract path after "cd " keyword
	newPath := strings.TrimSpace(strings.TrimPrefix(command, "cd "))

	if newPath == "" {
		return errorFcall(fc, "cd: path required")
	}

	// Expand ~ to home directory
	if strings.HasPrefix(newPath, "~") {
		home := os.Getenv("HOME")
		if home == "" {
			return errorFcall(fc, "cd: cannot expand ~ (HOME not set)")
		}
		newPath = strings.Replace(newPath, "~", home, 1)
	}

	// Clean the path
	newPath = filepath.Clean(newPath)

	// Convert to absolute path
	absPath, err := filepath.Abs(newPath)
	if err != nil {
		return errorFcall(fc, fmt.Sprintf("cd: invalid path: %v", err))
	}

	// Verify directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return errorFcall(fc, fmt.Sprintf("cd: %v", err))
	}
	if !info.IsDir() {
		return errorFcall(fc, fmt.Sprintf("cd: %s is not a directory", absPath))
	}

	// Load metadata from new directory
	newNotes, err := disk.LoadAll(absPath)
	if err != nil {
		return errorFcall(fc, fmt.Sprintf("cd: failed to load directory: %v", err))
	}

	// Update server state atomically
	s.mu.Lock()
	s.denoteDir = absPath
	s.notes = newNotes
	// Clear filter state when switching directories
	s.filterQuery = ""
	s.filteredNotes = nil
	s.mu.Unlock()

	return &plan9.Fcall{
		Type:  plan9.Rwrite,
		Tag:   fc.Tag,
		Count: uint32(len(fc.Data)),
	}
}

// parseFilterQuery parses a filter command query into Filter objects
// Handles quoted strings for titles: title:"my title" or title:'my title'
// Multiple filters space-separated: tag:journal !tag:draft
func parseFilterQuery(query string) ([]*metadata.Filter, error) {
	var filters []*metadata.Filter

	// Split by spaces while respecting quotes
	parts := splitRespectingQuotes(query)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Use existing NewFilter function
		filt, err := metadata.NewFilter(part)
		if err != nil {
			return nil, fmt.Errorf("invalid filter '%s': %w", part, err)
		}
		filters = append(filters, filt)
	}

	if len(filters) == 0 {
		return nil, fmt.Errorf("no valid filters provided")
	}

	return filters, nil
}

// splitRespectingQuotes splits a string on spaces but preserves quoted strings
// Handles both single and double quotes
func splitRespectingQuotes(s string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		ch := s[i]

		switch ch {
		case '"', '\'':
			if !inQuote {
				// Start quote
				inQuote = true
				quoteChar = ch
				current.WriteByte(ch)
			} else if ch == quoteChar {
				// End quote (matching)
				inQuote = false
				quoteChar = 0
				current.WriteByte(ch)
			} else {
				// Different quote char inside quotes
				current.WriteByte(ch)
			}
		case ' ':
			if inQuote {
				current.WriteByte(ch)
			} else {
				if current.Len() > 0 {
					result = append(result, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteByte(ch)
		}
	}

	// Add final token
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}
