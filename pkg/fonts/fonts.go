package fonts

import (
	_ "embed"
)

//go:embed AtkinsonHyperlegibleNext-Regular.ttf
var atkinsonRegularData []byte

//go:embed AtkinsonHyperlegibleNext-Bold.ttf
var atkinsonBoldData []byte

// GetAtkinsonRegular returns the embedded regular Atkinson Hyperlegible Next font data
func GetAtkinsonRegular() []byte {
	return atkinsonRegularData
}

// GetAtkinsonBold returns the embedded bold Atkinson Hyperlegible Next font data
func GetAtkinsonBold() []byte {
	return atkinsonBoldData
}
