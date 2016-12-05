package main

import (
	"fmt"
	"log"
	"math"
	"time"
)

type t_ctx struct {
	mean [2]float64
	m2   [2]float64
	n    [2]float64
}

const chunck_size = 16
const number_measurements = 10000

const t_threshold_bananas = 500 // test failed, with overwhelming probability
const t_threshold_moderate = 10

const number_percentiles = 100
const enough_measurements = 10000 // may be handled by the Go benchmark package later
const number_tests = 1 + number_percentiles + 1

var percentiles [number_percentiles]int64
var t [number_tests]t_ctx

func prepare_percentiles(ticks []int64) {
	for i := 0; i < number_percentiles; i++ {
		percentiles[i] = percentile(
			ticks, 1-(math.Pow(0.5, float64(10*(i+1))/float64(number_percentiles))))
	}
}

func measure(ticks []int64, input_data []string) {
	for i := 0; i < number_measurements; i++ {
		ticks[i] = time.Now().UnixNano()
		do_one_computation(input_data[i])
	}
	ticks[number_measurements] = time.Now().UnixNano()
}

func differentiate(exec_times []int64, ticks []int64) {
	for i := 0; i < number_measurements; i++ {
		exec_times[i] = ticks[i+1] - ticks[i]
	}
}

func update_statistics(exec_times []int64, classes []int) {
	for i := 0; i < number_measurements; i++ {
		difference := exec_times[i]

		if difference < 0 {
			continue // the cpu cycle counter overflowed
		}

		// do a t-test on the execution time
		t_push(&t[0], float64(difference), classes[i])

		// do a t-test on cropped execution times, for several cropping thresholds.
		for crop_index := 0; crop_index < number_percentiles; crop_index++ {
			if difference < percentiles[crop_index] {
				t_push(&t[crop_index+1], float64(difference), classes[i])
			}
		}

		// do a second-order test (only if we have more than 10000 measurements).
		// Centered product pre-processing.
		if t[0].n[0] > 10000 {
			centered := float64(difference) - t[0].mean[classes[i]]
			t_push(&t[1+number_percentiles], centered*centered, classes[i])
		}
	}
}

func t_push(ctx *t_ctx, x float64, class int) {
	if !(class == 0 || class == 1) {
		log.Fatalln("Error, wrong class in t_push")
	}
	ctx.n[class]++
	// Welford method for computing online variance
	// in a numerically stable way.
	// see Knuth Vol 2
	var delta float64
	delta = x - ctx.mean[class]
	ctx.mean[class] = ctx.mean[class] + delta/ctx.n[class]
	ctx.m2[class] = ctx.m2[class] + delta*(x-ctx.mean[class])

}

func wrap_report(x *t_ctx) {
	if x.n[0] > enough_measurements {
		var tval float64
		tval = t_compute(x)
		fmt.Printf("got t=%4.2f\n", tval)
	} else {
		fmt.Printf(" (not enough measurements %f)\n", x.n[0])
	}
}

func t_compute(ctx *t_ctx) float64 {
	vars := [2]float64{0.0, 0.0}
	var den, t_value, num float64

	vars[0] = ctx.m2[0] / (ctx.n[0] - 1)
	vars[1] = ctx.m2[1] / (ctx.n[1] - 1)
	num = (ctx.mean[0] - ctx.mean[1])
	den = math.Sqrt(vars[0]/ctx.n[0] + vars[1]/ctx.n[1])
	t_value = num / den

	return t_value
}

func max_test() int {
	ret := 0
	var max float64
	max = 0.0
	for i := 0; i < number_tests; i++ {
		if t[i].n[0] > enough_measurements {
			var x float64
			x = math.Abs(t_compute(&t[i]))
			if max < x {
				max = x
				ret = i
			}
		}
	}
	return ret
}

func report() {

	/*for (size_t i = 0; i < number_tests; i++) {
		    //fmt.Printf("traces %zu %f\n", i, t[i]->n[0] +  t[i]->n[1]);
	}*/
	fmt.Printf("\n\n")
	fmt.Printf("first order\n")
	wrap_report(&t[0])
	fmt.Printf("cropped\n")
	for i := 0; i < number_percentiles; i++ {
		wrap_report(&t[i+1])
	}
	fmt.Printf("second order\n")
	wrap_report(&t[1+number_percentiles])

	mt := max_test()
	max_t := math.Abs(t_compute(&t[mt]))
	number_traces_max_t := t[mt].n[0] + t[mt].n[1]
	max_tau := max_t / number_traces_max_t

	fmt.Printf("meas: %7.2lf M, ", (number_traces_max_t / 1e6))
	if number_traces_max_t < enough_measurements {
		fmt.Printf("not enough measurements (%.0f still to go).\n", enough_measurements-number_traces_max_t)
		return
	}

	/*
	* max_t: the t statistic value
	* max_tau: a t value normalized by number of measurements.
	*          this way we can compare max_tau taken with different
	*          number of measurements. This is sort of "distance
	*          between distributions", independent of number of
	*          measurements.
	* (5/tau)^2: how many measurements we would need to barely
	*            detect the leak, if present. "barely detect the
	*            leak" = have a t value greater than 5.
	 */
	fmt.Printf("max t: %+7.2f, max tau: %.2e, (5/tau)^2: %.2e.",
		max_t,
		max_tau,
		float64(5*5)/float64(max_tau*max_tau))

	if max_t > t_threshold_bananas {
		fmt.Printf(" Definitely not constant time.\n")
		return
	}
	if max_t > t_threshold_moderate {
		fmt.Printf(" Probably not constant time.\n")
	}
	if max_t < t_threshold_moderate {
		fmt.Printf(" For the moment, maybe constant time.\n")
	}

}

func doit() {
	ticks := make([]int64, number_measurements+1)
	exec_times := make([]int64, number_measurements)
	classes := make([]int, number_measurements)
	input_data := make([]string, number_measurements)

	prepare_inputs(input_data, classes)
	measure(ticks, input_data)
	differentiate(exec_times, ticks) // inplace

	if percentiles[number_percentiles-1] == 0 {
		prepare_percentiles(exec_times)
	} else {
		update_statistics(exec_times, classes)
		report()
	}
}

func main() {
	percentiles[1] = 12

	fmt.Println("vim-go", percentiles[1])
}
