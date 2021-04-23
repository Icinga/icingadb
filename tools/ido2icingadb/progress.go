package main

import (
	"github.com/vbauerster/mpb/v6"
	"github.com/vbauerster/mpb/v6/decor"
	"time"
)

// task abstracts a series of progress bars for task name within group.
type task struct {
	group      *mpb.Progress
	name       string
	currentJob *mpb.Bar
}

// startTrackableJob starts a new progress bar for job name with the progress done of total.
// incr increases the progress.
func (t *task) startTrackableJob(name string, total, done int64) (incr func()) {
	opts := []mpb.BarOption{
		mpb.BarFillerClearOnComplete(),
		mpb.PrependDecorators(
			decor.Name(t.name+":", decor.WC{W: len(t.name) + 2, C: decor.DidentRight}),
			decor.Name(name, decor.WCSyncSpaceR),
			decor.Percentage(decor.WC{W: 5}),
		),
		mpb.AppendDecorators(decor.EwmaETA(decor.ET_STYLE_GO, 60, decor.WC{W: 4})),
	}

	t.supersedePreviousJob(&opts)

	bar := t.group.AddBar(total, opts...)
	start := time.Now()
	t.currentJob = bar

	bar.SetCurrent(done)

	return func() {
		prev := start
		now := time.Now()
		start = now

		bar.IncrBy(1)
		bar.DecoratorEwmaUpdate(now.Sub(prev))
	}
}

// startOneShotJob starts a new progress bar for job name with the progress 0/1.
func (t *task) startOneShotJob(name string) {
	opts := []mpb.BarOption{
		mpb.BarFillerClearOnComplete(),
		mpb.PrependDecorators(
			decor.Name(t.name, decor.WC{W: len(t.name) + 1, C: decor.DidentRight}),
			decor.Name(name, decor.WCSyncSpaceR),
		),
	}

	t.supersedePreviousJob(&opts)
	t.currentJob = t.group.AddBar(1, opts...)
}

// finish finishes the series of progress bars t.
func (t *task) finish() {
	if t.currentJob != nil {
		t.finishCurrent()
	}
}

// supersedePreviousJob deduplicates startTrackableJob and startOneShotJob.
func (t *task) supersedePreviousJob(opts *[]mpb.BarOption) {
	if t.currentJob != nil {
		*opts = append(*opts, mpb.BarQueueAfter(t.currentJob))
		t.finishCurrent()
	}
}

// finish finishes the current progress bar of t.
func (t *task) finishCurrent() {
	if !t.currentJob.Completed() {
		t.currentJob.SetTotal(t.currentJob.Current(), true)
	}
}
