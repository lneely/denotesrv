package disk

import (
	"bytes"
	"denotesrv/pkg/metadata"
	"os/exec"
	"path/filepath"
	"strings"
)

func FindBacklinks(targetID, denoteDir string, allNotes metadata.Results) metadata.Results {
	searchPattern := "denote:" + targetID

	duCmd := exec.Command("9", "du", "-a")
	duCmd.Dir = denoteDir

	awkCmd := exec.Command("9", "awk", "$2 ~ /\\.(md|org|txt)$/ {print $2}")

	xargsCmd := exec.Command("9", "xargs", "9", "grep", "-l", searchPattern)
	xargsCmd.Dir = denoteDir

	duOut, err := duCmd.StdoutPipe()
	if err != nil {
		return nil
	}
	awkCmd.Stdin = duOut

	awkOut, err := awkCmd.StdoutPipe()
	if err != nil {
		return nil
	}
	xargsCmd.Stdin = awkOut

	var output bytes.Buffer
	xargsCmd.Stdout = &output

	if err := duCmd.Start(); err != nil {
		return nil
	}
	if err := awkCmd.Start(); err != nil {
		return nil
	}
	if err := xargsCmd.Start(); err != nil {
		return nil
	}

	duCmd.Wait()
	awkCmd.Wait()
	xargsCmd.Wait()

	result := strings.TrimSpace(output.String())
	if result == "" {
		return nil
	}

	var results metadata.Results
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		line = strings.TrimPrefix(line, "./")
		m := metadata.ParseFilename(filepath.Join(denoteDir, line))
		if m.Identifier == "" {
			continue
		}

		for _, note := range allNotes {
			if note.Identifier == m.Identifier {
				results = append(results, note)
				break
			}
		}
	}

	return results
}
