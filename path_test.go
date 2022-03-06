//go:build !windows

package doccurator

import "testing"

func TestPleasantPath(t *testing.T) {
	type args struct {
		absolute     string
		root         string
		wd           string
		collapseRoot bool
		omitDotSlash bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "NextToFileInRoot_1", args: args{absolute: "/my/lib/file", root: "/my/lib", wd: "/my/lib", collapseRoot: true, omitDotSlash: true}, want: "file"},
		{name: "NextToFileInRoot_2", args: args{absolute: "/my/lib/file", root: "/my/lib", wd: "/my/lib", collapseRoot: true, omitDotSlash: false}, want: "./file"},
		{name: "NextToFileInRoot_3", args: args{absolute: "/my/lib/file", root: "/my/lib", wd: "/my/lib", collapseRoot: false, omitDotSlash: true}, want: "file"},
		{name: "NextToFileInRoot_4", args: args{absolute: "/my/lib/file", root: "/my/lib", wd: "/my/lib", collapseRoot: false, omitDotSlash: false}, want: "./file"},
		{name: "FileInSubFromRoot_1", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my/lib", collapseRoot: true, omitDotSlash: true}, want: "sub/file"},
		{name: "FileInSubFromRoot_2", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my/lib", collapseRoot: true, omitDotSlash: false}, want: "./sub/file"},
		{name: "FileInSubFromRoot_3", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my/lib", collapseRoot: false, omitDotSlash: true}, want: "sub/file"},
		{name: "FileInSubFromRoot_4", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my/lib", collapseRoot: false, omitDotSlash: false}, want: "./sub/file"},
		{name: "FileInRootFromSub_1", args: args{absolute: "/my/lib/file", root: "/my/lib", wd: "/my/lib/sub", collapseRoot: true, omitDotSlash: true}, want: "../file"},
		{name: "FileInRootFromSub_2", args: args{absolute: "/my/lib/file", root: "/my/lib", wd: "/my/lib/sub", collapseRoot: true, omitDotSlash: false}, want: "../file"},
		{name: "FileInRootFromSub_3", args: args{absolute: "/my/lib/file", root: "/my/lib", wd: "/my/lib/sub", collapseRoot: false, omitDotSlash: true}, want: "../file"},
		{name: "FileInRootFromSub_4", args: args{absolute: "/my/lib/file", root: "/my/lib", wd: "/my/lib/sub", collapseRoot: false, omitDotSlash: false}, want: "../file"},
		{name: "FileInRootFromDeep", args: args{absolute: "/my/lib/file", root: "/my/lib", wd: "/my/lib/sub/deep", collapseRoot: true, omitDotSlash: false}, want: "../../file"},
		{name: "NextToFileInSub_1", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my/lib/sub", collapseRoot: true, omitDotSlash: true}, want: "file"},
		{name: "NextToFileInSub_2", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my/lib/sub", collapseRoot: true, omitDotSlash: false}, want: "./file"},
		{name: "NextToFileInSub_3", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my/lib/sub", collapseRoot: false, omitDotSlash: true}, want: "file"},
		{name: "NextToFileInSub_4", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my/lib/sub", collapseRoot: false, omitDotSlash: false}, want: "./file"},
		{name: "OutsideLibrary_1", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/", collapseRoot: true, omitDotSlash: true}, want: "lib://sub/file"},
		{name: "OutsideLibrary_2", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/", collapseRoot: true, omitDotSlash: false}, want: "lib://sub/file"},
		{name: "OutsideLibrary_3", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/", collapseRoot: false, omitDotSlash: true}, want: "/my/lib/sub/file"},
		{name: "OutsideLibrary_4", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/", collapseRoot: false, omitDotSlash: false}, want: "/my/lib/sub/file"},
		{name: "BarelyOutsideLibrary_1", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my", collapseRoot: true, omitDotSlash: true}, want: "lib://sub/file"},
		{name: "BarelyOutsideLibrary_2", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my", collapseRoot: true, omitDotSlash: false}, want: "lib://sub/file"},
		{name: "BarelyOutsideLibrary_3", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my", collapseRoot: false, omitDotSlash: true}, want: "/my/lib/sub/file"},
		{name: "BarelyOutsideLibrary_4", args: args{absolute: "/my/lib/sub/file", root: "/my/lib", wd: "/my", collapseRoot: false, omitDotSlash: false}, want: "/my/lib/sub/file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pleasantPath(tt.args.absolute, tt.args.root, tt.args.wd, tt.args.collapseRoot, tt.args.omitDotSlash); got != tt.want {
				t.Errorf("pleasantPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
