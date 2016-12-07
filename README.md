# go-dudect

This is a toy implementation in Go of [Dudect](https://github.com/oreparaz/dudect), almost directly translated from C.

Its only purpose is to play around with the DecryptOAEP function to see if the signal caused by the leftPad timing discrepancies is enough to leak being given DecryptOAEP noise. Long story, short: no it doesn't. So Manger's attack would not apply there, unless someone finds another detectable discprancy or a better distinguisher, of course.  (Note that Dudect's aim isn't to be a distinguisher, just to assess the existence of a timing leak and that it's not because it didn't find one there isn't one. The same applies here.)

[dudect.go](dudect.go) ideas' and design's are coming from Oscar Reparaz, Josep Balasch and Ingrid Verbauwhede, all credit goes to them.

## To use it with your code

Simply write a file containing your function, a `func prepare_inputs() (input_data [][]byte, classes []int)` function returning the input data and its classes and a `func do_one_computation(data []byte)` function using your function on the given input, then do a `make filename`, _et voilà_ you can try it with your Go code, natively without having to use any kind of wrapper in C or whatever.
