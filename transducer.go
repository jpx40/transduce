package transduce

import (
	"fmt"
	"math/rand"
	"reflect"
)

type Transducer func(ReducerFn) ReducerFn

func (t Transducer) Compose(other Transducer) Transducer {
	return func(s ReducerFn) ReducerFn {
		return t(other(s))
	}
}

type initFn func() interface{}
type resultFn func(result interface{}) interface{}
type reducingFn func(result, input interface{}) interface{}

type ReducerFn interface {
	Init() interface{}
	Result(result interface{}) interface{}
	Step(result, input interface{}) interface{}
}

type reducer struct {
	init   initFn
	result resultFn
	step   reducingFn
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

func Reduced(val interface{}) interface{} {
	return &reduced{val: val}
}

func EnsureReduced(val interface{}) interface{} {
	if IsReduced(val) {
		return val
	}
	return Reduced(val)
}

func IsReduced(val interface{}) bool {
	_, ok := val.(*reduced)
	return ok
}

func Unreduced(val interface{}) interface{} {
	if IsReduced(val) {
		return val.(*reduced).Deref()
	}
	return val
}

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

func RandomSample(prob float64) Transducer {
	return Filter(func(_ interface{}) bool {
		return rand.Float64() < prob
	})
}

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
// reduce must be of the form func(rfn func(r rT, i iT) rT, r rT, i iT) rT.
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

type predicate func(x interface{}) bool

func wrapPredicate(f interface{}) predicate {
	switch fn := f.(type) {
	case predicate:
		return fn
	default:
		return func(in interface{}) bool {
			return apply(f, in).(bool)
		}
	}
}

type mapper func(interface{}) interface{}

func wrapMapper(f interface{}) mapper {
	switch fn := f.(type) {
	case mapper:
		return fn
	default:
		return func(in interface{}) interface{} {
			return apply(f, in)
		}
	}
}

type reduceFn func(interface{}, interface{}, interface{}) interface{}

func wrapReduce(f interface{}) reduceFn {
	switch fn := f.(type) {
	case reduceFn:
		return fn
	default:
		return func(rfn, res, in interface{}) interface{} {
			return apply(f, rfn, res, in)
		}
	}
}

func wrapReducing(f interface{}) reducingFn {
	switch fn := f.(type) {
	case reducingFn:
		return fn
	default:
		return func(res, in interface{}) interface{} {
			return apply(f, res, in)
		}
	}
}

func wrapInit(f interface{}) initFn {
	switch fn := f.(type) {
	case initFn:
		return fn
	default:
		return func() interface{} {
			return apply(f)
		}
	}

}

func wrapResult(f interface{}) resultFn {
	switch fn := f.(type) {
	case resultFn:
		return fn
	default:
		return func(result interface{}) interface{} {
			return apply(f, result)
		}
	}
}

func apply(f interface{}, args ...interface{}) interface{} {
	fnv := reflect.ValueOf(f)
	fnt := fnv.Type()
	argvs := make([]reflect.Value, len(args))
	for i, arg := range args {
		if arg == nil {
			fnint := fnt.In(i)
			fnink := fnint.Kind()
			switch fnink {
			case reflect.Chan, reflect.Func,
				reflect.Interface, reflect.Map,
				reflect.Ptr, reflect.Slice:
				argvs[i] = reflect.Zero(fnint)
			default:
				// this will cause a panic but that is what is
				// intended
				argvs[i] = reflect.ValueOf(arg)
			}
		} else {
			argvs[i] = reflect.ValueOf(arg)
		}
	}
	outvs := fnv.Call(argvs)
	switch len(outvs) {
	case 0:
		return nil
	case 1:
		return outvs[0].Interface()
	default:
		outs := make([]interface{}, len(outvs))
		for i, outv := range outvs {
			outs[i] = outv.Interface()
		}
		return outs
	}
}
