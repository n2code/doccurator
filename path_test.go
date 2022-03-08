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

func TestIsChildOf(t *testing.T) {
	type args struct {
		child  string
		parent string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "DeepLibFolder1", args: args{child: "/my/lib/sub/leaf", parent: "/my/lib"}, want: true},
		{name: "DeepLibFolder2", args: args{child: "/my/lib/sub", parent: "/my/lib"}, want: true},
		{name: "DeepLibFolder3", args: args{child: "/my/lib", parent: "/my/lib"}, want: false},
		{name: "DeepLibFolder4", args: args{child: "/my", parent: "/my/lib"}, want: false},
		{name: "DeepLibFolder5", args: args{child: "/", parent: "/my/lib"}, want: false},
		{name: "DeepLibFolder6", args: args{child: "/my/other/sub/leaf", parent: "/my/lib"}, want: false},
		{name: "DeepLibFolder7", args: args{child: "/my/other/sub", parent: "/my/lib"}, want: false},
		{name: "DeepLibFolder8", args: args{child: "/my/other", parent: "/my/lib"}, want: false},
		{name: "LibFolderInRoot1", args: args{child: "/lib/sub/leaf", parent: "/lib"}, want: true},
		{name: "LibFolderInRoot2", args: args{child: "/lib/sub", parent: "/lib"}, want: true},
		{name: "LibFolderInRoot3", args: args{child: "/lib", parent: "/lib"}, want: false},
		{name: "LibFolderInRoot4", args: args{child: "/", parent: "/lib"}, want: false},
		{name: "RootAsLib1", args: args{child: "/sub/leaf", parent: "/"}, want: true},
		{name: "RootAsLib2", args: args{child: "/sub", parent: "/"}, want: true},
		{name: "RootAsLib3", args: args{child: "/", parent: "/"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isChildOf(tt.args.child, tt.args.parent); got != tt.want {
				t.Errorf("isChildOf() = %v, want %v", got, tt.want)
			}
		})
	}
}
