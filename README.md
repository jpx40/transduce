# Transduce

[![GoDoc](https://godoc.org/jsouthworth.net/go/transduce?status.svg)](https://godoc.org/jsouthworth.net/go/transduce)

Transduce implements a set of [transducers](https://www.youtube.com/watch?v=6mTbuzafcII) in go. The goal was mainly to play with these, but I ended up writing a pretty full featured lazy sequence library on top of them. These transducers use reflection to allow one to write natural looking code to pass to them. The types are then enforced at runtime. This tradeoff seems to work well in practice.

So what is a transducer? A transducer is a function that transforms a reducing function.

Ok, then what is a reducer? A reducer is set of functions of  0, 1 and 2 arity respectively. In go this is represented by an interface of three methods and a constructor to build a reified version of this from 3 passed in functions. This allows for a more functional style when writing most transducers. Arity 0 is known as Init and is used to retrieve an initial value for a reduction. Arity 1 is known as Result and returns the result of the reduction. This is typically the identity function and the Completing constructor will create this from only the Arity 2 function. Arity 2 is the Step function, this computes one step of the reduction.

## Getting started
```
go get jsouthworth.net/go/transduce
```

## Usage

The full documentation is available at
[jsouthworth.net/go/transduce](https://jsouthworth.net/go/transduce)

## License

This project is licensed under the MIT License - see [LICENSE](LICENSE)

## Acknowledgments

* Rich Hickey's [StrangeLoop talk](https://www.youtube.com/watch?v=6mTbuzafcII) got me interested in doing this.
* [Clojure Docs](https://clojure.org/reference/transducers) for helping me understand how they were intended to function a bit better.
* [transducers-go](https://github.com/sdboyer/transducers-go) pointed out some ways in which an implementation does feel natural in go. Hopefully I've addressed some of these.

## TODO

* [ ] Find a good way to process channels with transducers.
