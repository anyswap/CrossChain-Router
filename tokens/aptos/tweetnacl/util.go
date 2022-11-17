package tweetnacl

import (
	"C"
	"reflect"
	"unsafe"
)

// Convenience function to convert a byte array (or slice) to a 'C' pointer.  The
// function handles the zero length array case, which the normal version doesn't -
// i.e. (*C.uchar)(unsafe.Pointer(&msg[0])) will crash with a panic when msg is a
// zero length array.
//
// The commented out version which returns nil for a zero length array is somewhat
// less bizarre and works as well.
//
// Ref. https://groups.google.com/forum/#!topic/golang-nuts/NNBdjztWquo
//
func makePtr(array []byte) *C.uchar {
	return (*C.uchar)(unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&array)).Data))

	// if len(array) == 0 {
	// 	 return nil
	// } else {
	// 	 return (*C.uchar)(unsafe.Pointer(&array[0]))
	// }
}
