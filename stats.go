package main

import (
	"log"
	_ "math"
	"sort"
)

// Let us fullfill the Sort interface:
type Int64ToSort []int64

func (s Int64ToSort) Len() int { return len(s) }

func (s Int64ToSort) Less(i, j int) bool { return s[i] < s[j] }

func (s Int64ToSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func mean(x []int64) float64 {
	var m float64
	for _, v := range x {
		m += float64(v)
	}
	m = m / float64(len(x))
	return m
}

func median(x []int64) int64 {
	sort.Sort(Int64ToSort(x))
	m := (len(x) + 1) / 2
	return x[m]
}

func percentile(x []int64, perc float64) int64 {
	val := int(perc * float64(len(x)))
	if len(x) >= val || 0 >= val {
		log.Fatalln("Error, percentile should be smaller than 1 and bigger than 0.")
	}
	sort.Sort(Int64ToSort(x))
	return x[val]
}
