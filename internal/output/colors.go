package output

type SgrModifier string //ANSI control sequences: Select Graphic Rendition (SGR)

const (
	NormalIntensity   SgrModifier = "\x1B[22m"
	BoldIntensity     SgrModifier = "\x1B[1m"
	FaintIntensity    SgrModifier = "\x1B[2m"
	Invert            SgrModifier = "\x1B[7m"
	Red               SgrModifier = "\x1B[31m"
	Green             SgrModifier = "\x1B[32m"
	Yellow            SgrModifier = "\x1B[33m"
	Magenta           SgrModifier = "\x1B[35m"
	Cyan              SgrModifier = "\x1B[36m"
	DefaultForeground SgrModifier = "\x1B[39m"
	Reset             SgrModifier = "\x1B[0m"
)
