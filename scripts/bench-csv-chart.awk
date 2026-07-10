BEGIN {
	FS = ","
	FPAT = "([^,]*)|(\"([^\"]|\"\")*\")"
	out = out == "" ? "_tests/benchmark/benchmark-comparison.svg" : out
	title = title == "" ? "SQLite Driver Benchmark Comparison" : title
	width = 1180
	left = 330
	right = 40
	top = 92
	groupH = 56
	barH = 16
	gap = 4
	plotW = width - left - right
}

function clean(v) {
	if (v ~ /^"/) {
		v = substr(v, 2, length(v) - 2)
		gsub(/""/, "\"", v)
	}
	return v
}

function esc(v) {
	gsub(/&/, "\\&amp;", v)
	gsub(/</, "\\&lt;", v)
	gsub(/>/, "\\&gt;", v)
	gsub(/"/, "\\&quot;", v)
	return v
}

function short_bench(v) {
	sub(/^Benchmark/, "", v)
	return v
}

function fmt_ns(ns) {
	if (ns >= 1000000000) return sprintf("%.2fs", ns / 1000000000)
	if (ns >= 1000000) return sprintf("%.1fms", ns / 1000000)
	if (ns >= 1000) return sprintf("%.1fus", ns / 1000)
	return sprintf("%.0fns", ns)
}

function log10(v) {
	return log(v) / log(10)
}

FNR == 1 {
	next
}

{
	driver = clean($1)
	bench = clean($6)
	ns = clean($9) + 0
	if (driver == "" || bench == "" || ns <= 0) next

	if (!(driver in seenDriver)) {
		seenDriver[driver] = 1
		drivers[++driverN] = driver
	}
	if (!(bench in seenBench)) {
		seenBench[bench] = 1
		benches[++benchN] = bench
	}

	time[bench, driver] = ns
	rows[bench, driver] = clean($10)
	bytes[bench, driver] = clean($17)
	allocs[bench, driver] = clean($18)
	l = log10(ns)
	if (!haveScale || l < minLog) minLog = l
	if (!haveScale || l > maxLog) maxLog = l
	haveScale = 1
}

END {
	if (driverN < 1 || benchN < 1) {
		print "no benchmark rows found" > "/dev/stderr"
		exit 1
	}
	if (maxLog == minLog) maxLog = minLog + 1

	height = top + benchN * groupH + 86
	print "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"" width "\" height=\"" height "\" viewBox=\"0 0 " width " " height "\">" > out
	print "<rect width=\"100%\" height=\"100%\" fill=\"#ffffff\"/>" >> out
	print "<style>text{font-family:Arial,Helvetica,sans-serif;fill:#172033}.title{font-size:22px;font-weight:700}.note{font-size:12px;fill:#667085}.bench{font-size:12px;font-weight:700}.label{font-size:11px}.axis{stroke:#d0d5dd;stroke-width:1}.grid{stroke:#eaecf0;stroke-width:1}.barlabel{font-size:11px;fill:#344054}</style>" >> out
	print "<text class=\"title\" x=\"24\" y=\"34\">" esc(title) "</text>" >> out
	print "<text class=\"note\" x=\"24\" y=\"56\">Horizontal bars use log10(ns/op); shorter is faster. Labels show actual ns/op converted to time.</text>" >> out

	colors[1] = "#2563eb"
	colors[2] = "#dc6803"
	colors[3] = "#039855"
	colors[4] = "#7a5af8"

	legendX = left
	for (d = 1; d <= driverN; d++) {
		color = colors[((d - 1) % 4) + 1]
		print "<rect x=\"" legendX "\" y=\"26\" width=\"12\" height=\"12\" rx=\"2\" fill=\"" color "\"/>" >> out
		print "<text class=\"label\" x=\"" legendX + 18 "\" y=\"36\">" esc(drivers[d]) "</text>" >> out
		legendX += 150
	}

	for (tick = int(minLog); tick <= int(maxLog) + 1; tick++) {
		x = left + ((tick - minLog) / (maxLog - minLog)) * plotW
		if (x < left || x > left + plotW) continue
		print "<line class=\"grid\" x1=\"" x "\" y1=\"" top - 18 "\" x2=\"" x "\" y2=\"" height - 62 "\"/>" >> out
		print "<text class=\"note\" text-anchor=\"middle\" x=\"" x "\" y=\"" height - 40 "\">10^" tick " ns</text>" >> out
	}
	print "<line class=\"axis\" x1=\"" left "\" y1=\"" height - 62 "\" x2=\"" left + plotW "\" y2=\"" height - 62 "\"/>" >> out

	for (b = 1; b <= benchN; b++) {
		bench = benches[b]
		y = top + (b - 1) * groupH
		print "<text class=\"bench\" text-anchor=\"end\" x=\"" left - 14 "\" y=\"" y + 18 "\">" esc(short_bench(bench)) "</text>" >> out

		for (d = 1; d <= driverN; d++) {
			driver = drivers[d]
			ns = time[bench, driver]
			if (ns <= 0) continue
			l = log10(ns)
			barW = ((l - minLog) / (maxLog - minLog)) * plotW
			if (barW < 2) barW = 2
			barY = y + 2 + (d - 1) * (barH + gap)
			color = colors[((d - 1) % 4) + 1]
			print "<rect x=\"" left "\" y=\"" barY "\" width=\"" barW "\" height=\"" barH "\" rx=\"3\" fill=\"" color "\"/>" >> out
			print "<text class=\"barlabel\" x=\"" left + barW + 6 "\" y=\"" barY + 12 "\">" esc(fmt_ns(ns)) "</text>" >> out
		}

		if (driverN == 2 && time[bench, drivers[1]] > 0 && time[bench, drivers[2]] > 0) {
			ratio = time[bench, drivers[2]] / time[bench, drivers[1]]
			print "<text class=\"note\" x=\"" left "\" y=\"" y + 48 "\">" esc(drivers[2]) "/" esc(drivers[1]) ": " sprintf("%.2fx", ratio) " time</text>" >> out
		}
	}

	print "</svg>" >> out
	print out
}
