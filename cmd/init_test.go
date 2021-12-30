// Copyright The RAI Inc.
// The RAI Authors
package cmd

import "testing"

func Test_getProjectName(t *testing.T) {
	type args struct {
		pkgName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "bean",
			args:    args{"bean"},
			want:    "bean",
			wantErr: false,
		},
		{
			name:    "",
			args:    args{""},
			want:    "",
			wantErr: true,
		},
		{
			name:    "/bean",
			args:    args{"/bean"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "github.com/retail-ai-inc/test-bean",
			args:    args{"github.com/retail-ai-inc/test-bean"},
			want:    "test-bean",
			wantErr: false,
		},
		{
			name:    "github.com/.retail-ai-inc/test-bean",
			args:    args{"github.com/.retail-ai-inc/test-bean"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "github.com/retail-ai-inc/test-bean.",
			args:    args{"github.com/retail-ai-inc/test-bean."},
			want:    "",
			wantErr: true,
		},
		{
			name:    "EXAMPL~1.COM/test-bean",
			args:    args{"EXAMPL~1.COM/test-bean"},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getProjectName(tt.args.pkgName)
			if (err != nil) != tt.wantErr {
				t.Errorf("getProjectName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getProjectName() = %v, want %v", got, tt.want)
			}
		})
	}
}
