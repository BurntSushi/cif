package cif

// CIF represents an entire CIF file.
type CIF struct {
	// Version, when present in the source file, contains the version of the
	// specification that the file was generated for. e.g., "CIF_1.1".
	Version string

	// Blocks maps data block names to corresponding data blocks.
	// Note that all data block names are stored in lowercase.
	Blocks map[string]*DataBlock
}

// Block represents the structure of any block-like section in a CIF file.
// It corresponds either to a data block or a save frame.
type Block struct {
	// The name of this block.
	Name string

	// Items maps data tags to values. Data tags that are part of a "loop_"
	// declaration are not included here.
	Items map[string]Value

	// Loops maps data tags to tables defined by a "loop_" declaration. Namely,
	// each data tag in a "loop_" maps to precisely the same loop object.
	// For example, if a "loop_" introduces the data tag "_cats" with values
	// "Cauchy" and "Plato", then the following code accesses that list of
	// cats:
	//
	//	loop := someBlock.Loops["cats"]
	//	cats := loop.Get("cats").Strings()
	//
	// Notice that the key "cats" is used twice. The first time it's used to
	// get the loop object, while the second time its used is to get the
	// actual column of data.
	Loops map[string]*Loop
}

// DataBlock represents a data block in a CIF file.
type DataBlock struct {
	// The name and data items are stored in a block.
	Block

	// Frames maps save frame names to corresponding save frames.
	// Note that all save frame names are stored in lowercase.
	Frames map[string]*SaveFrame
}

// SaveFrame represents a save frame in a CIF file.
type SaveFrame struct {
	// The name and data items are stored in a block.
	Block
}

// Loop represents a single table of data within a block.
type Loop struct {
	// Columns maps data tag names to their position in the table (starting
	// at 0).
	Columns map[string]int

	// Values corresponds to the columns of data in the table. Namely, each
	// ValueLoop is a single column of data.
	Values []ValueLoop
}

// Get is a convenience method for retrieving a column of data
// corresponding to the data tag given in a particular block.
//
// The underlying type of ValueLoop is guaranteed to be []string, []int or
// []float64. See the documentation for ValueLoop for more details.
//
// Note that all data tags are stored in lowercase.
func (lp *Loop) Get(name string) ValueLoop {
	return lp.Values[lp.Columns[name]]
}
