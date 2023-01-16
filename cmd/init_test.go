// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
