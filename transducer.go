package transduce

import (
	"fmt"
	"math/rand"
	"reflect"

	"jsouthworth.net/go/dyn"
)

// Transducers take a ReducerFn and return a new ReducerFn transforming it
// for additional functionalliy.
type Transducer func(ReducerFn) ReducerFn

// Compose is function composition of two Transducers.
func (t Transducer) Compose(other Transducer) Transducer {
	return func(s ReducerFn) ReducerFn {
		return t(other(s))
	}
}

// ReducerFn represents a reducing function. A reducer is set of functions of  0, 1 and 2 arity respectively. Here this is represented by an interface of three methods and a constructor to build a reified version of this from 3 passed in functions. This allows for a more functional style when writing most transducers.
type ReducerFn interface {
	// Arity 0 is known as Init and is used to retrieve
	// an initial value for a reduction
	Init() interface{}
	// Arity 1 is known as Result and returns the result of the reduction.
	// This is typically the identity function and the Completing
	// constructor will create this from only the Arity 2 function/
	Result(result interface{}) interface{}
	// Arity 2 is the Step function, this computes one
	// step of the reduction.
	Step(result, input interface{}) interface{}
}

type reducer struct {
	init   func() interface{}
	result func(result interface{}) interface{}
	step   func(result, input interface{}) interface{}
}

func (t *reducer) Init() interface{} {
	return t.init()
}
func (t *reducer) Result(result interface{}) interface{} {
	return t.result(result)
}
func (t *reducer) Step(result, input interface{}) interface{} {
	return t.step(result, input)
}

// Reducer constructs a ReducerFn from the three functions. This allows
// a functional defintion style for ReducerFns. init is any function matching
// the signature func() T. result is any function matching the signature
// func(x iT) oT. step is any function matching the signature
// func(r rT, i iT) rT. Reflection is used to call thes functions unless they
// are of the non-specialized types func()interface{},
// func(interface{})interface{}, and func(interface,interface{})interface{}
// respectively.
func Reducer(
	init interface{},
	result interface{},
	step interface{},
) ReducerFn {
	return &reducer{
		init:   wrapInit(init),
		result: wrapResult(result),
		step:   wrapReducing(step),
	}
}

// Reducing returns a Transducer that only modifies the reducing step function.
// rfn is the new reducing function and must match the signature
// func(a rT, i iT) rT.
func Reducing(rfn interface{}) Transducer {
	reducer := wrapReducing(rfn)
	return func(step ReducerFn) ReducerFn {
		return Reducer(
			func() interface{} {
				return step.Init()
			},
			func(result interface{}) interface{} {
				return step.Result(result)
			},
			reducer,
		)
	}
}

// Completing returns a ReducerFn with a standard Init and Result function.
// rfn is the new reduction step and must match the signature
// func(a rT, i iT) rT.
func Completing(rfn interface{}) ReducerFn {
	reducer := wrapReducing(rfn)
	return Reducer(
		func() interface{} {
			return nil
		},
		func(result interface{}) interface{} {
			return result
		},
		reducer,
	)
}

type reduced struct {
	val interface{}
}

func (r *reduced) Deref() interface{} {
	return r.val
}

func (r *reduced) String() string {
	return fmt.Sprintf("Reduced(%v)", r.val)
}

// Reduced returns a value wrapped such that it is marked
// to signify termination of the processing.
func Reduced(val interface{}) interface{} {
	return &reduced{val: val}
}

// EnsureReduced wraps a value if it is not currently Reduced.
func EnsureReduced(val interface{}) interface{} {
	if IsReduced(val) {
		return val
	}
	return Reduced(val)
}

// IsReduced returns whether or not the value is Reduced.
func IsReduced(val interface{}) bool {
	_, ok := val.(*reduced)
	return ok
}

// Unreduced unwraps the original value from a Reduced value.
// If the value is not currently wrapped then it is returned
// unmodified.
func Unreduced(val interface{}) interface{} {
	if IsReduced(val) {
		return val.(*reduced).Deref()
	}
	return val
}

// Replace returns a transducer that will replace elements of a stream
// with corresponding elements in smap. smap may be one of the following
// types interface{ Find(interface{})(interface{},bool) },
// map[interface{}]interface{}, map[kT]vT.
func Replace(smap interface{}) Transducer {
	return Map(func(in interface{}) interface{} {
		switch m := smap.(type) {
		case interface {
			Find(in interface{}) (interface{}, bool)
		}:
			out, ok := m.Find(in)
			if !ok {
				return in
			}
			return out
		case map[interface{}]interface{}:
			out, ok := m[in]
			if !ok {
				return in
			}
			return out
		default:
			out := reflect.ValueOf(smap).
				MapIndex(reflect.ValueOf(in))
			if !out.IsValid() {
				return in
			}
			return out.Interface()
		}
	})
}

// Map returns a transducer that will replace elements in the stream
// with a corresponding element from the range of f. f must match the
// signature func(iT) oT.
func Map(f interface{}) Transducer {
	mapFn := wrapMapper(f)
	return func(rf ReducerFn) ReducerFn {
		return Reducing(
			func(result, input interface{}) interface{} {
				return rf.Step(result, mapFn(input))
			},
		)(rf)
	}
}

// Filter returns a transducer that will skip elements in a stream
// if they do not match the predicate. pred must match the signature
// func(i iT) bool.
func Filter(pred interface{}) Transducer {
	predFn := wrapPredicate(pred)
	return func(rf ReducerFn) ReducerFn {
		return Reducing(
			func(result, input interface{}) interface{} {
				if predFn(input) {
					return rf.Step(result, input)
				}
				return result
			},
		)(rf)
	}
}

// Remove returns a transducer that will skip elements in a stream
// if they do match the predicate. pred must match the signature
// func(i iT) bool.
func Remove(pred interface{}) Transducer {
	predFn := wrapPredicate(pred)
	return func(rf ReducerFn) ReducerFn {
		return Reducing(
			func(result, input interface{}) interface{} {
				if !predFn(input) {
					return rf.Step(result, input)
				}
				return result
			},
		)(rf)
	}
}

// Take returns a stateful transducer that will end processing of a
// stream after n elements.
func Take(n int) Transducer {
	return func(rf ReducerFn) ReducerFn {
		count := n
		return Reducing(
			func(result, input interface{}) interface{} {
				current := count
				next := count - 1
				count = next
				if current > 0 {
					result = rf.Step(result, input)
				}
				if next > 0 {
					return result
				}
				return EnsureReduced(result)
			},
		)(rf)
	}
}

// TakeWhile returns a transducer that will end processing of
// a stream if the predicate becomes false. pred must match the signature
// func(i iT) bool.
func TakeWhile(pred interface{}) Transducer {
	predFn := wrapPredicate(pred)
	return func(rf ReducerFn) ReducerFn {
		return Reducing(
			func(result, input interface{}) interface{} {
				if predFn(input) {
					return rf.Step(result, input)
				}
				return EnsureReduced(result)
			},
		)(rf)
	}
}

// TakeNth returns a stateful transducer that will skip all elements of
// a stream whose index is not divisible by n.
func TakeNth(n int) Transducer {
	return func(rf ReducerFn) ReducerFn {
		count := 0
		return Reducing(
			func(result, input interface{}) interface{} {
				count++
				if count%n == 0 {
					return rf.Step(result, input)
				}
				return result
			},
		)(rf)
	}
}

// Drop returns a stateful transducer that will skip the first n elements
// of a stream and process the rest.
func Drop(n int) Transducer {
	return func(rf ReducerFn) ReducerFn {
		dropped := 0
		return Reducing(
			func(result, input interface{}) interface{} {
				if dropped < n {
					dropped++
					return result
				}
				return rf.Step(result, input)
			},
		)(rf)
	}
}

// DropWhile returns a stateful transducer that will skip all elements
// of the stream until the predicate returns false and process the rest.
// pred must match the signature func(i iT) bool.
func DropWhile(pred interface{}) Transducer {
	predFn := wrapPredicate(pred)
	return func(rf ReducerFn) ReducerFn {
		drop := true
		return Reducing(
			func(result, input interface{}) interface{} {
				if drop && predFn(input) {
					return result
				}
				drop = false
				return rf.Step(result, input)
			},
		)(rf)
	}
}

// Keep returns a transducer that will keep all non-nil elements of a stream
// and skip the rest. f must be of the signature func(i iT) oT; oT must be
// nilable or all elements will be kept.
func Keep(f interface{}) Transducer {
	mapFn := wrapMapper(f)
	return func(rf ReducerFn) ReducerFn {
		return Reducing(
			func(result, input interface{}) interface{} {
				ret := mapFn(input)
				if ret != nil {
					return rf.Step(result, ret)
				}
				return result
			},
		)(rf)
	}
}

// KeepIndexed returns a stateful transducer that will keep all
// non-nil elements of a stream and skip the rest. f must be of the signature
// func(idx int, i iT) oT; oT must be nilable or all elements will be kept.
func KeepIndexed(f interface{}) Transducer {
	var fn func(int, interface{}) interface{}
	switch v := f.(type) {
	case func(int, interface{}) interface{}:
		fn = v
	default:
		fn = func(idx int, val interface{}) interface{} {
			return apply(f, idx, val)
		}
	}
	return func(rf ReducerFn) ReducerFn {
		index := 0
		return Reducing(
			func(result, input interface{}) interface{} {
				ret := fn(index, input)
				index++
				if ret != nil {
					return rf.Step(result, ret)
				}
				return result
			},
		)(rf)
	}
}

// Dedupe returns a stateful transducer that will deduplicate
// adjacent elements of a stream.
func Dedupe() Transducer {
	return func(rf ReducerFn) ReducerFn {
		var prior interface{}
		return Reducing(
			func(result, input interface{}) interface{} {
				if prior == input {
					return result
				}
				prior = input
				return rf.Step(result, input)
			},
		)(rf)
	}
}

// RandomSample returns a transducer that will process a random sampling of
// data from the stream, all other data is skipped.
func RandomSample(prob float64) Transducer {
	return Filter(func(_ interface{}) bool {
		return rand.Float64() < prob
	})
}

// PartitionBy returns a stateful transducer that will partition a stream
// when f returns a different result than previous result. f must match
// the signature func(i iT) oT.
func PartitionBy(f interface{}) Transducer {
	mapFn := wrapMapper(f)
	return func(rf ReducerFn) ReducerFn {
		part := []interface{}{}
		var mark int
		var prior interface{} = &mark
		return Reducer(
			func() interface{} {
				return rf.Init()
			},
			func(result interface{}) interface{} {
				ret := result
				if len(part) > 0 {
					cpy := make([]interface{}, len(part))
					copy(cpy, part)
					part = []interface{}{}
					ret = Unreduced(rf.Step(result, cpy))
				}
				return rf.Result(ret)
			},
			func(result, input interface{}) interface{} {
				val := mapFn(input)
				pval := prior
				prior = val
				if pval == &mark || pval == val {
					part = append(part, input)
					return result
				} else {
					cpy := make([]interface{}, len(part))
					copy(cpy, part)
					part = []interface{}{}
					ret := rf.Step(result, cpy)
					if !IsReduced(ret) {
						part = append(part, input)
					}
					return ret
				}
			},
		)
	}
}

// PartitionAll returns a stateful transducer that will partition a stream
// into n sized buckets. The final bucket may be smaller than n if the number of
// elements is not divisible by n.
func PartitionAll(n int) Transducer {
	return func(rf ReducerFn) ReducerFn {
		part := make([]interface{}, 0, n)
		return Reducer(
			func() interface{} {
				return rf.Init()
			},
			func(result interface{}) interface{} {
				ret := result
				if len(part) > 0 {
					cpy := make([]interface{}, len(part))
					copy(cpy, part)
					part = make([]interface{}, 0, n)
					ret = rf.Step(result, cpy)
				}
				return rf.Result(ret)
			},
			func(result, input interface{}) interface{} {
				part = append(part, input)
				if n == len(part) {
					cpy := make([]interface{}, len(part))
					copy(cpy, part)
					part = make([]interface{}, 0, n)
					return rf.Step(result, cpy)
				}
				return result
			},
		)
	}
}

// Interpose is a stateful transducer that will place an element
// between each element in a stream.
func Interpose(sep interface{}) Transducer {
	return func(rf ReducerFn) ReducerFn {
		started := false
		return Reducing(
			func(result, input interface{}) interface{} {
				if started {
					sepr := rf.Step(result, sep)
					if IsReduced(sepr) {
						return sepr
					}
					return rf.Step(sepr, input)
				}
				started = true
				return rf.Step(result, input)
			},
		)(rf)
	}
}

// Cat returns a transducer that will concatenate the contents of each input.
// reduce is a function that can traverse a sequence. It must be of the form
// func(rfn func(r rT, i iT) rT, r rT, i iT) rT. This allows multiple stragies
// for traversing any type of sequence.
func Cat(reduce interface{}) Transducer {
	reduceFn := wrapReduce(reduce)
	return func(rf ReducerFn) ReducerFn {
		rrf := PreservingReduced(rf)
		return Reducing(
			func(result, input interface{}) interface{} {
				return reduceFn(rrf.Step, result, input)
			},
		)(rf)
	}
}

// Mapcat produces a transducer that composes Map and Cat. This
// transducer will call f on each element of a collection and then concat
// the results of multiple collections. reduce must be of the form:
// func(rfn func(r rT, i iT) rT, r rT, i iT) rT, f must be of the form
// func(in iT) oT and will be called with reflection unless they are the
// non specialized cases.
func Mapcat(reduce interface{}, f interface{}) Transducer {
	return Compose(Map(f), Cat(reduce))
}

// PreservingReduced is a transducer that will preserve the termination
// status of a value by encapsulating an already reduced value in another
// reduced value.
func PreservingReduced(rf ReducerFn) ReducerFn {
	return Reducer(rf.Init, rf.Result,
		func(result, input interface{}) interface{} {
			ret := rf.Step(result, input)
			if IsReduced(ret) {
				return Reduced(ret)
			}
			return ret
		})
}

// Compose composes transducers together. ts[0](ts[1](...ts[n]))
func Compose(ts ...Transducer) Transducer {
	switch len(ts) {
	case 0:
		return func(s ReducerFn) ReducerFn {
			return s
		}
	case 1:
		return ts[0]
	default:
		var out = ts[0]
		for _, t := range ts[1:] {
			out = out.Compose(t)
		}
		return out
	}
}

func wrapPredicate(f interface{}) func(x interface{}) bool {
	switch fn := f.(type) {
	case func(x interface{}) bool:
		return fn
	default:
		return func(in interface{}) bool {
			return apply(f, in).(bool)
		}
	}
}

func wrapMapper(f interface{}) func(interface{}) interface{} {
	switch fn := f.(type) {
	case func(interface{}) interface{}:
		return fn
	default:
		return func(in interface{}) interface{} {
			return apply(f, in)
		}
	}
}

func wrapReduce(f interface{}) func(interface{}, interface{}, interface{}) interface{} {
	switch fn := f.(type) {
	case func(interface{}, interface{}, interface{}) interface{}:
		return fn
	default:
		return func(rfn, res, in interface{}) interface{} {
			return apply(f, rfn, res, in)
		}
	}
}

func wrapReducing(f interface{}) func(r, i interface{}) interface{} {
	switch fn := f.(type) {
	case func(result, input interface{}) interface{}:
		return fn
	default:
		return func(res, in interface{}) interface{} {
			return apply(f, res, in)
		}
	}
}

func wrapInit(f interface{}) func() interface{} {
	switch fn := f.(type) {
	case func() interface{}:
		return fn
	default:
		return func() interface{} {
			return apply(f)
		}
	}

}

func wrapResult(f interface{}) func(interface{}) interface{} {
	switch fn := f.(type) {
	case func(interface{}) interface{}:
		return fn
	default:
		return func(result interface{}) interface{} {
			return apply(f, result)
		}
	}
}

func apply(f interface{}, args ...interface{}) interface{} {
	return dyn.Apply(f, args...)
}
