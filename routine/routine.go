package routine

func Run(fn, recover func()) {
	go func() {
		defer recover()
		fn()
	}()
}
