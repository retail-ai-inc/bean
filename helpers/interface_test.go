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

package helpers

import (
	"reflect"
	"testing"
)

func TestConvertInterfaceToSlice(t *testing.T) {
	type args struct {
		value interface{}
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		// TODO: Add test cases.
		{name: "slice", args: args{value: []int{1, 2, 3}}, want: []int{1, 2, 3}},
		{name: "array", args: args{value: [...]int{1, 2, 3}}, want: [...]int{1, 2, 3}},
		{name: "int", args: args{value: 1}, want: []interface{}{1}},
		{name: "string", args: args{value: "Tom"}, want: []interface{}{"Tom"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertInterfaceToSlice(tt.args.value); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertInterfaceToSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertInterfaceToBool(t *testing.T) {
	type args struct {
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
		{name: "bool", args: args{value: true}, want: true, wantErr: false},
		{name: "string 1", args: args{value: "1"}, want: true, wantErr: false},
		{name: "string 2", args: args{value: "2"}, want: false, wantErr: true},
		{name: "string 0", args: args{value: "0"}, want: false, wantErr: false},
		{name: "string f", args: args{value: "f"}, want: false, wantErr: false},
		{name: "int 1", args: args{value: 1}, want: true, wantErr: false},
		{name: "int 2", args: args{value: int32(2)}, want: false, wantErr: true},
		{name: "int 0", args: args{value: int64(0)}, want: false, wantErr: false},
		{name: "uint 1", args: args{value: uint(1)}, want: true, wantErr: false},
		{name: "uint 2", args: args{value: uint64(2)}, want: false, wantErr: true},
		{name: "uint 0", args: args{value: uint32(0)}, want: false, wantErr: false},
		{name: "float 1", args: args{value: 1.0}, want: true, wantErr: false},
		{name: "float 2", args: args{value: 2.0}, want: false, wantErr: true},
		{name: "float 0", args: args{value: 0.0}, want: false, wantErr: false},
		{name: "float 0.1", args: args{value: 1.1}, want: false, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertInterfaceToBool(tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertInterfaceToBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertInterfaceToBool() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertInterfaceToFloat(t *testing.T) {
	type args struct {
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    float64
		wantErr bool
	}{
		// TODO: Add test cases.
		{name: "string 1", args: args{value: "1"}, want: 1, wantErr: false},
		{name: "string 2.16", args: args{value: "2.16"}, want: 2.16, wantErr: false},
		{name: "string f", args: args{value: "f"}, want: 0, wantErr: true},
		{name: "int 1", args: args{value: 1}, want: 1, wantErr: false},
		{name: "uint 1", args: args{value: uint(1)}, want: 1, wantErr: false},
		{name: "float 1", args: args{value: 1.0}, want: 1, wantErr: false},
		{name: "float 0.1", args: args{value: 0.1}, want: 0.1, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertInterfaceToFloat(tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertInterfaceToFloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertInterfaceToFloat() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertInterfaceToString(t *testing.T) {
	type args struct {
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{name: "string", args: args{value: "abc"}, want: "abc", wantErr: false},
		{name: "bool", args: args{value: true}, want: "true", wantErr: false},
		{name: "int", args: args{value: 3}, want: "3", wantErr: false},
		{name: "uint", args: args{value: 4}, want: "4", wantErr: false},
		{name: "float", args: args{value: 2.10}, want: "2.1", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertInterfaceToString(tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertInterfaceToString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertInterfaceToString() got = %v, want %v", got, tt.want)
			}
		})
	}
}
