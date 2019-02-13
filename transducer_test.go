package transduce

func ExampleReducer() {
	// This reducer will drop all inputs and complete the reduction
	_ = Reducer(
		func() interface{} {
			return nil
		},
		func(result interface{}) interface{} {
			return result
		},
		func(result, input interface{}) interface{} {
			return result
		},
	)
}

func ExampleReducing() {
	// This transducer will drop all inputs
	_ = Reducing(func(result, input interface{}) interface{} {
		return result
	})
}

func ExampleCompleting() {
	// This transducer will drop all inputs and complete the reduction
	_ = Completing(func(result, input interface{}) interface{} {
		return result
	})
}

func ExampleReplace() {
	_ = Replace(map[int]string{
		2:  "two",
		6:  "six",
		18: "eighteen",
	})
}

func ExampleMap() {
	_ = Map(func(x int) int {
		return x + 1
	})
}

func ExampleFilter() {
	_ = Filter(func(x int) bool {
		return x%2 == 0
	})
}

func ExampleRemove() {
	_ = Remove(func(x interface{}) bool {
		_, isString := x.(string)
		return isString
	})
}

func ExampleTake() {
	_ = Take(11)
}

func ExampleTakeWhile() {
	_ = TakeWhile(func(x interface{}) bool {
		return x != 300
	})
}

func ExampleTakeNth() {
	_ = TakeNth(1)
}

func ExampleDrop() {
	_ = Drop(1)
}

func ExampleDropWhile() {
	_ = DropWhile(func(x interface{}) bool {
		_, isString := x.(string)
		return isString
	})
}

func ExampleKeep() {
	_ = Keep(func(x int) interface{} {
		if x%2 != 0 {
			return x * x
		}
		return nil
	})
}

func ExampleKeepIndexed() {
	_ = KeepIndexed(func(i, x int) interface{} {
		if i%2 == 0 {
			return i * x
		}
		return nil
	})
}

func ExampleDedupe() {
	_ = Dedupe()
}

func ExampleRandomSample() {
	_ = RandomSample(1.0)
}

func ExamplePartitionBy() {
	_ = PartitionBy(func(x int) bool {
		return x > 7
	})
}

func ExamplePartitionAll() {
	_ = PartitionAll(3)
}

func ExampleInterpose() {
	_ = Interpose("/")
}

func ExampleCat() {
	_ = Cat(func(rfn, result, input interface{}) interface{} {
		return apply(rfn, result, input)
	})
}

func ExampleMapcat() {
	_ = Mapcat(func(rfn, result, input interface{}) interface{} {
		return apply(rfn, result, input)
	}, func(x int) int { return x + x })
}

func ExampleCompose() {
	_ = Compose(
		Map(func(x int) int {
			return x + 1
		}),
		Filter(func(x int) bool {
			return x%2 == 0
		}),
	)
}
