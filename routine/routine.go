package routine

// Run runs a function in a goroutine and recovers from any panics.
func Run(fn, recover func()) {
	go func() {
		defer recover()
		fn()
	}()
}
