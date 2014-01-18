package cif

import (
	"strconv"
	"strings"
)

type loopValues struct {
	name string
	strs []string

	// typ corresponds to the most restrictive type that can describe every
	// value in the list. In particular, it is always string if a loop column
	// has any combination of types (including missing and omitted).
	typ itemType
}

// convertLoopValues ensures that the Go type of each column of values in a
// loop is either []string, []int or []float64. The []int type is *only* used
// when all values are integers. The []float64 type is *only* used when all
// values are either integers or floats. The []string type is used in all other
// circumstances.
func (p *parser) convertLoopValues(b Block, vals []loopValues) *Loop {
	lp := &Loop{
		Columns: make(map[string]int, len(vals)),
		Values:  make([]ValueLoop, len(vals)),
	}
	for i, val := range vals {
		switch val.typ {
		case itemDataOmitted, itemDataMissing, itemDataString:
			strs := make([]string, len(val.strs))
			copy(strs, val.strs)
			lp.Values[i] = AsValues(strs)
		case itemDataInteger:
			nums := make([]int, len(val.strs))
			for j, str := range val.strs {
				if str == "." || str == "?" {
					nums[j] = 0
					continue
				}

				n, err := strconv.Atoi(str)
				if err != nil {
					p.errf("Could not parse '%s' as integer: %s", str, err)
				}
				nums[j] = n
			}
			lp.Values[i] = AsValues(nums)
		case itemDataFloat:
			nums := make([]float64, len(val.strs))
			for j, str := range val.strs {
				if str == "." || str == "?" {
					nums[j] = 0
					continue
				}

				n, err := strconv.ParseFloat(str, 64)
				if err != nil {
					p.errf("Could not parse '%s' as float: %s", str, err)
				}
				nums[j] = n
			}
			lp.Values[i] = AsValues(nums)
		default:
			p.errf("Expected value for data tag '%s' in block '%s', but "+
				"got a '%s' instead.", val.name, b.Name, val.typ)
		}
	}
	for i, val := range vals {
		lp.Columns[val.name] = i
		b.Loops[val.name] = lp
	}
	return lp
}

func (p *parser) parseLoop(b Block) item {
	loopLine := p.line // save start line for error messages

	// Check that there's at least one data tag. Then slurp up any remaining
	// data tags.
	t := p.next()
	if t.typ != itemDataTag {
		p.errf("After 'loop_' declaration, there must be at least one "+
			"data tag, but found '%s' instead.", t.typ)
	}
	vals := make([]loopValues, 0, 5)
	for i := 0; t.typ == itemDataTag; t, i = p.next(), i+1 {
		name := strings.ToLower(t.val)
		p.assertUniqueTag(b, name)
		vals = append(vals, loopValues{
			name: name,
			strs: make([]string, 0, 10),
			typ:  itemDataNone,
		})
	}

	// Check that there is at least one value. Then slurp up any remaining
	// values. Note that we read everything as strings initially, but if a list
	// ends up being all integers or all floats, then we convert them.
	if !isValueType(t.typ) {
		p.errf("After 'loop_' declaration, there must be at least one "+
			"data tag and at least one value, but found '%s' instead of a "+
			"value.", t.typ)
	}
	count := 0 // must end up being a multiple of len(loop.Columns)
	for i := 0; isValueType(t.typ); t, i, count = p.next(), i+1, count+1 {
		column := i % len(vals)
		vals[column].strs = append(vals[column].strs, t.val)
		if vals[column].typ == itemDataNone {
			vals[column].typ = t.typ
		} else if vals[column].typ != t.typ && !isNull(t.typ) {
			// If there is a mix of integers and floats, that's OK. But use
			// float.
			if (isInteger(vals[column].typ) && isFloat(t.typ)) ||
				(isFloat(vals[column].typ) && isInteger(t.typ)) {
				vals[column].typ = itemDataFloat
			} else {
				vals[column].typ = itemDataString
			}
		}
	}
	if count%len(vals) != 0 {
		p.errf("There are %d values in loop (starting on line %d), which is "+
			"not a multiple of the number of columns in the loop (%d).",
			count, loopLine, len(vals))
	}
	p.convertLoopValues(b, vals)
	return t
}
