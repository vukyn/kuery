package conv

// ToPointer converts a value to a pointer.
//
// Example:
//
//	fmt.Println(ToPointer(123)) // Output: 0xc0000b6018
func ToPointer[T any](i T) *T {
	return &i
}
