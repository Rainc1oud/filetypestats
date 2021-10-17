package filetypestats

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ppenguin/filetypestats/types"
)

// scaffold generated with vscode "Add unit tests for function"
func TestWalkFileTypeStatsDB(t *testing.T) {
	type args struct {
		scanDirs []string
		dbfile   string
	}

	pwd, _ := os.Getwd()
	hdir, _ := os.UserHomeDir()
	scandir := filepath.Join(hdir, "Documents")

	dbfile := filepath.Join(pwd, "testdb.sqlite")
	// defer os.Remove(dbfile)

	tests := []struct {
		name    string
		args    args
		want    types.FileTypeStats
		wantErr bool
	}{
		{
			name:    "simple-scan",
			args:    args{scanDirs: []string{scandir}, dbfile: dbfile},
			want:    types.FileTypeStats{}, // TODO: get the correct value from a proven scan
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WalkFileTypeStatsDB(tt.args.scanDirs, tt.args.dbfile)
			if (err != nil) != tt.wantErr {
				t.Errorf("WalkFileTypeStatsDB() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WalkFileTypeStatsDB() = %v, want %v", got, tt.want)
			}
		})
	}
}
