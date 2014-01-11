package cif

// Value denotes any value in a data item. Its underlying type is guaranteed
// to be string, int or float64.
// Note that this includes omitted (".") and unknown ("?") data. Both are
// stored as strings.
type Value interface {
	// String returns this value as a string. If its underlying type is
	// numeric, then 0 is returned.
	String() string

	// Int returns this value as an integer. If its underlying type is
	// a float, then an int conversion is performed (which may fail).
	// If its underlying type is a string, then the empty string is returned.
	Int() int

	// Float returns this value as a float64. If its underlying type is
	// an integer, then it is converted to a float.
	// If its underlying type is a string, then the empty string is returned.
	Float() float64

	// Raw provides the underlying string, int or float64 value. The interface
	// returned may be used in a type switch.
	// (A Value itself is not amenable to type switching, since the types that
	// satisfy it in this package are not exported.)
	Raw() interface{}
}

type cifString string

func (cs cifString) String() string   { return string(cs) }
func (cs cifString) Int() int         { return 0 }
func (cs cifString) Float() float64 { return 0 }
func (cs cifString) Raw() interface{} { return string(cs) }

type cifInt int

func (ci cifInt) String() string   { return "" }
func (ci cifInt) Int() int         { return int(ci) }
func (ci cifInt) Float() float64 { return float64(ci) }
func (ci cifInt) Raw() interface{} { return int(ci) }

type cifFloat float64

func (cf cifFloat) String() string   { return "" }
func (cf cifFloat) Int() int         { return int(cf) }
func (cf cifFloat) Float() float64 { return float64(cf) }
func (cf cifFloat) Raw() interface{} { return float64(cf) }

// AsValue returns a value that satisfies the Value interface if v
// has type string, int or float. If v has any other type, this function will
// panic.
//
// This function should only be used when constructing values for writing
// CIF data.
func AsValue(v interface{}) Value {
	switch v := v.(type) {
	case string:
		return cifString(v)
	case int:
		return cifInt(v)
	case float64:
		return cifFloat(v)
	}
	panic(sf("Type '%T' cannot be represented as a CIF value.", v))
}

// ValueLoop denotes a single column of data in a table. Its underlying type
// is guaranteed to be []string, []int or []float64.
//
// Note that []int and []float64 are only used when the column can be
// interpreted as a homogenous array of data (containing all integers, all
// floats or a mixture of integers and floats where all integers are converted
// to floats). If any other type of value is found in the column, then all
// values are represented as strings.
//
// Note that this interface does not have a Raw method since a nil slice is
// returned if and only if the method called does not correspond to its
// underlying type. (In particular, the CIF specification guarantees that
// every column in a loop has at least one value, which means all columns must
// be non-nil.)
type ValueLoop interface{
	// Strings returns this value as a []string. If its underlying type is
	// not []string, then nil is returned.
	Strings() []string

	// Ints returns this value as a []int. If its underlying type is
	// not []int, then nil is returned.
	Ints() []int

	// Floats returns this value as a []float64. If its underlying type is
	// not []float64, then nil is returned.
	Floats() []float64
}

type cifStrings []string

func (cs cifStrings) Strings() []string   { return []string(cs) }
func (cs cifStrings) Ints() []int         { return nil }
func (cs cifStrings) Floats() []float64 { return nil }

type cifInts []int

func (ci cifInts) Strings() []string   { return nil }
func (ci cifInts) Ints() []int         { return []int(ci) }
func (ci cifInts) Floats() []float64 { return nil }

type cifFloats []float64

func (cf cifFloats) Strings() []string   { return nil }
func (cf cifFloats) Ints() []int         { return nil }
func (cf cifFloats) Floats() []float64 { return []float64(cf) }

// AsValues returns a value that satisfies the ValueLoop interface if v
// has type []string, []int or []float. If v has any other type, this function
// will panic.
//
// This function should only be used when constructing values for writing
// CIF data.
func AsValues(v interface{}) ValueLoop {
	switch v := v.(type) {
	case []string:
		return cifStrings(v)
	case []int:
		return cifInts(v)
	case []float64:
		return cifFloats(v)
	}
	panic(sf("Type '%T' cannot be represented as a CIF loop column.", v))
}

