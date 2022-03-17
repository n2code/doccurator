package library

import "github.com/n2code/doccurator/internal/output"

func ColorForStatus(status PathStatus) output.SgrModifier {
	switch status {
	case Tracked, Removed:
		return output.DefaultForeground
	case Untracked:
		return output.Cyan //color of progress
	case Touched, Moved:
		return output.Green //color of good news (harmless)
	case Modified:
		return output.Yellow //color of attention
	case Duplicate, Obsolete:
		return output.Magenta //color of waste
	case Error, Missing:
		return output.Red //color of trouble
	default:
		return output.DefaultForeground
	}
}
