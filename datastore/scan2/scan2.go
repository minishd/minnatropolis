// Utility package that tries to make scanning structs
// from databases simpler.
// It does its best to handle nested structs
// and nullable structs where it is able
package scan2

import (
	"reflect"
	"slices"
	"strconv"
)

type Rows interface {
	Scan(dest ...any) error
}

// var typScanner = reflect.TypeFor[sql.Scanner]()

func needsFlatten(typ reflect.Type) bool {
	_, ok := registry[typ]
	return ok
	// return typ.Kind() == reflect.Struct && !typ.Implements(typScanner)
}

// Wrapper type that we use to handle nullable structs
// with required fields
type requiredFieldProxy struct {
	field, scanned reflect.Value
}

// A pointer to a structure
// with all required fields that must be null-checked
type nullZone struct {
	pstruct reflect.Value
	fields  []*requiredFieldProxy
}

var registry = make(map[reflect.Type]struct{})

func Register2[T any]() {
	typ := reflect.TypeFor[T]()
	registry[typ] = struct{}{}
}

func flatten(val reflect.Value, pthisStruct *reflect.Value) (fields []any, nzs []nullZone) {
	// Intermediary slice of fields
	// We can have a struct field that is another struct,
	// and structs have multiple fields,
	// so we do this to make it easier to handle....
	var ifields [][]any

	// Set up null-zone if we are in the context of
	// a nullable struct with required fields
	var nz nullZone
	if pthisStruct != nil {
		nz.pstruct = *pthisStruct
	}

	for sf, fval := range val.Fields() {
		// Get struct tag
		idx_ := sf.Tag.Get("scan2")
		if idx_ == "" {
			// If it wasn't given a struct tag,
			// we will ignore it ..
			continue
		}

		// Parse tag
		idx, err := strconv.Atoi(idx_)
		if err != nil {
			panic("struct tag is not an int")
		}

		// Negatives are invalid
		if idx < 0 {
			panic("struct tag must not be negative")
		}

		// If that index isn't in the array yet,
		// create it
		if idx >= len(ifields) {
			// Make enough blank slices to reach
			// the index we want to set
			needed := (idx + 1) - len(ifields)
			filler := make([][]any, needed)
			// Add to array
			ifields = append(ifields, filler...)
		} else if ifields[idx] != nil {
			// That index already exists in the array
			// as a non-null slice
			// If we wrote anything there we would
			// overwrite a pre-existing field ..
			panic("duplicate struct tag")
		}

		// The struct tag seems valid

		// Do we need to flatten this field?
		if needsFlatten(sf.Type) {
			// Yes, flatten and add all resulting fields
			var fnzs []nullZone
			ifields[idx], fnzs = flatten(fval, pthisStruct)
			nzs = append(nzs, fnzs...)
			continue
		}

		// Is this field a pointer?
		if fval.Kind() == reflect.Pointer {
			// Yes, so we must initialise it
			// Create new value of underlying type
			utyp := sf.Type.Elem()
			puval := reflect.New(utyp)
			// Set the pointer's value to the value we created
			fval.Set(puval)

			// Also do we need to flatten that value?
			if needsFlatten(utyp) {
				// Yes
				uval := reflect.Indirect(puval)
				var fnzs []nullZone
				ifields[idx], fnzs = flatten(uval, &fval)
				nzs = append(nzs, fnzs...)
				continue
			}

			// No we don't, continue on normally..
		}

		// Is this a required field of a nullable struct?
		if pthisStruct != nil && sf.Type.Kind() != reflect.Pointer {
			// Make a pointer value of the struct field's type
			// We will scan the value to this field,
			// and if it is not null, copy it to the struct
			ptyp := reflect.PointerTo(sf.Type)
			pval := reflect.New(ptyp).Elem()

			// Add an RFP we can check later
			rfp := &requiredFieldProxy{
				field:   fval,
				scanned: pval,
			}
			nz.fields = append(nz.fields, rfp)

			// Override fval with our new pointer
			fval = pval
		}

		// It is not a struct
		// That means it is only one value,
		// so add it to fields as a 1-element slice
		ifields[idx] = []any{fval.Addr().Interface()}
	}

	// Make sure there aren't any missing fields
	// (This would happen if you defined fields 0 and 2
	// 	 but not 1 for example)
	if slices.ContainsFunc(ifields, func(f []any) bool {
		return f == nil
	}) {
		panic("gap in fields defined by struct tag")
	}

	// Flatten, return ..
	fields = slices.Concat(ifields...)
	if len(nz.fields) != 0 {
		nzs = append(nzs, nz)
	}
	return
}

func Scan2[T any](r Rows) (*T, error) {
	typ := reflect.TypeFor[T]()
	pval := reflect.New(typ)
	val := reflect.Indirect(pval)
	fields, nzs := flatten(val, nil)

	if err := r.Scan(fields...); err != nil {
		return nil, err
	}

	// Make sure nullable structs with required fields
	// have all of the fields they need
	for _, nz := range nzs {
		for _, rfp := range nz.fields {
			// If what we scanned is nil,
			// nil out the struct ptr and quit
			if rfp.scanned.IsNil() {
				nz.pstruct.SetZero()
				break
			} else {
				// It's not nil, so copy to struct
				rfp.field.Set(rfp.scanned.Elem())
			}
		}
	}

	return pval.Interface().(*T), nil
}
