//go:build race

package stream_test

// memoryCues is the count of cues in the memory test. The race detector slows the pipeline by 20 times.
const memoryCues = 20000
