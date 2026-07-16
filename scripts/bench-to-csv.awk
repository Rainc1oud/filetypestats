BEGIN {
	cols[1] = "driver"
	cols[2] = "datetime"
	cols[3] = "goos"
	cols[4] = "goarch"
	cols[5] = "pkg"
	cols[6] = "benchmark"
	cols[7] = "procs"
	cols[8] = "iterations"
	cols[9] = "ns_per_op"
	cols[10] = "rows_per_s"
	cols[11] = "target_rows_per_op"
	cols[12] = "selected_rows_per_op"
	cols[13] = "table_rows"
	cols[14] = "initial_rows"
	cols[15] = "final_rows"
	cols[16] = "batch_cap"
	cols[17] = "bytes_per_op"
	cols[18] = "allocs_per_op"
	cols[19] = "raw"
	for (i = 1; i <= 19; i++) {
		printf "%s%s", cols[i], i == 19 ? ORS : ","
	}
}

function csv(s) {
	gsub(/"/, "\"\"", s)
	return "\"" s "\""
}

function reset_metrics() {
	ns_per_op = ""
	rows_per_s = ""
	target_rows_per_op = ""
	selected_rows_per_op = ""
	table_rows = ""
	initial_rows = ""
	final_rows = ""
	batch_cap = ""
	bytes_per_op = ""
	allocs_per_op = ""
}

/^goos:/ {
	goos = $2
	next
}

/^goarch:/ {
	goarch = $2
	next
}

/^pkg:/ {
	pkg = $2
	next
}

/^Benchmark/ {
	reset_metrics()
	raw = $0
	bench = $1
	procs = ""
	if (match(bench, /-[0-9]+$/)) {
		procs = substr(bench, RSTART + 1)
		bench = substr(bench, 1, RSTART - 1)
	}
	iterations = $2

	for (i = 3; i <= NF; i += 2) {
		value = $i
		unit = $(i + 1)
		if (unit == "ns/op") {
			ns_per_op = value
		} else if (unit == "rows/s") {
			rows_per_s = value
		} else if (unit == "target_rows/op") {
			target_rows_per_op = value
		} else if (unit == "selected_rows/op") {
			selected_rows_per_op = value
		} else if (unit == "table_rows") {
			table_rows = value
		} else if (unit == "initial_rows") {
			initial_rows = value
		} else if (unit == "final_rows") {
			final_rows = value
		} else if (unit == "batch_cap") {
			batch_cap = value
		} else if (unit == "B/op") {
			bytes_per_op = value
		} else if (unit == "allocs/op") {
			allocs_per_op = value
		}
	}

	printf "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
		csv(driver), csv(datetime), csv(goos), csv(goarch), csv(pkg), csv(bench),
		csv(procs), csv(iterations), csv(ns_per_op), csv(rows_per_s),
		csv(target_rows_per_op), csv(selected_rows_per_op), csv(table_rows),
		csv(initial_rows), csv(final_rows), csv(batch_cap), csv(bytes_per_op),
		csv(allocs_per_op), csv(raw)
}
