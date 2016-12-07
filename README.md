# go-dudect

This is a toy implementation in Go of the Dudect program, almost directly translated from C.

Its only purpose is to play around with the DecryptOAEP function to show that the signal caused by the leftPad timing discrepancies is not enough to exploit, being given DecryptOAEP noise. So Manger's attack would apply there, unless someone finds a detectable discprancy later, of course.

[dudect.go](dudect.go) ideas' and design's are coming from Oscar Reparaz, Josep Balasch and Ingrid Verbauwhede, all credit goes to them.

## To use it with your code

Simply write a file containing your function, a `func prepare_inputs() (input_data [][]byte, classes []int)` function returning the input data and its classes and a `func do_one_computation(data []byte)` function using your function on the given input, then do a `make filename`, _et voilà_ you can try it with your Go code!
