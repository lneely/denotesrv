package config

import "os"

// ============================================================
// CONFIGURATION: Default Denote Directory
//
// Change this value to set your default denote directory.
// This is used at program startup. After startup, use the
// Dsilo command to switch between directories dynamically.
// ============================================================
var DefaultDenoteDir = os.Getenv("HOME") + "/doc"

// Examples of alternative configurations:
// var DefaultDenoteDir = "/home/lkn/notes"
// var DefaultDenoteDir = os.Getenv("DENOTE_DIR")
