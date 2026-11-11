# Frame rates and time codes

Broadcast formats count time in frames. The SCC line `01:00:00;29` and the EBU STL time code `00 00 01 05` at 25 fps both name a frame. `model.FrameRate` holds the rate as an exact fraction, and the root package `stbox` re-exports the names below.

## Frame rates

A `FrameRate` has `Num` frames in `Den` seconds, and a `DropFrame` flag. `FrameRateFromFloat(23.976)` gives 2997/125, not 24000/1001. A format with a fixed table of rates maps an input rate to its table entry with `Near`. `Near` is true when the 2 rates differ by 0.01 frames per second or less. It ignores `DropFrame`.

| Name | Rate | Drop-frame | Labels per second |
|:--- |:--- |:---:|---:|
| `FrameRate23976` | 24000/1001 | no | 24 |
| `FrameRate24` | 24/1 | no | 24 |
| `FrameRate25` | 25/1 | no | 25 |
| `FrameRate2997` | 30000/1001 | no | 30 |
| `FrameRate2997DF` | 30000/1001 | yes | 30 |
| `FrameRate30` | 30/1 | no | 30 |
| `FrameRate50` | 50/1 | no | 50 |
| `FrameRate5994` | 60000/1001 | no | 60 |
| `FrameRate5994DF` | 60000/1001 | yes | 60 |
| `FrameRate60` | 60/1 | no | 60 |

The labels per second are the rate rounded to a whole number, half away from zero. `Labels` returns them. 25/2 gives 13.

`Validate` allows `DropFrame` only at 30000/1001, 2997/100, 60000/1001 and 2997/50. The methods below need a rate that passes `Validate`. With a zero rate they return 0 or the zero `Timecode` and never panic.

## Frames and durations

| Method | Result |
|:--- |:--- |
| `Frames(d)` | the frame nearest to `d`. 1 s at 30000/1001 gives frame 30 |
| `Duration(n)` | the exact time of frame `n`, to the nearest nanosecond. Frame 30 at 30000/1001 is 1.001 s |

Both use integer math, so 99999 hours at 60000/1001 does not overflow. A time that lies exactly on half a frame rounds up. 20 ms at 25 fps gives frame 1. A cue rounds a `Duration` result to whole milliseconds, so frame 15 at 30000/1001 gives 501 ms.

## The 2 time models

A format states which model it uses.

- **Frame-count model**: a label names a frame number, and the time is that number divided by the exact rate. SCC, MCC and iTT use it. At 29.97 fps the label `01:00:00:00` is 3603.6 s. `TimecodeOf` turns a frame into a label and `FrameOf` turns a label into a frame.
- **Clock model**: hours, minutes and seconds are clock time, and the frames add `frames / rate` seconds. EBU STL, PAC, Cavena 890, Spruce STL, DCP and CSV frame times use it. At 29.97 fps the label `01:00:00:00` is 3600 s. `ClockTimecode` turns a time into a label and `ClockDuration` turns a label into a time.

`ClockTimecode` rounds the time to whole milliseconds first. A frame at or above the labels per second rolls into the next second as frame 0. 1.996 s at 24000/1001 gives `00:00:02:00`.

## Drop-frame

At 29.97 fps a label count of 30 per second runs slower than the clock, by 3.6 s in an hour. A drop-frame label skips the labels `;00` and `;01` at the start of each minute, except every tenth minute. At 59.94 fps it skips 4 labels. `;` before the frames marks a drop-frame label.

| Rate | Frame | Label |
|:--- |---:|:--- |
| `FrameRate2997DF` | 1799 | `00:00:59;29` |
| `FrameRate2997DF` | 1800 | `00:01:00;02` |
| `FrameRate2997DF` | 17982 | `00:10:00;00` |
| `FrameRate2997DF` | 107892 | `01:00:00;00` |
| `FrameRate2997` | 108000 | `01:00:00:00` |

`FrameOf` reads the rate, not the `;` of the label. A label that drop-frame skips, such as `00:01:00;00`, gives the frame of the label before it. `FrameOf` and `ClockDuration` check no field. `DropFrameMode` selects drop-frame labels in the SCC and MCC writers. `DropFrameKeep` is the zero value and takes the mode that the reader stored.

## Parsing

`ParseTimecode` reads a label `HH:MM:SS:FF` or `HH:MM:SS;FF` and checks no range.

`ParseTime(s, r)` reads a time string and rounds it to whole milliseconds. It accepts 3 shapes.

| Shape | Examples |
|:--- |:--- |
| `h:mm:ss[.,]fraction` | `00:01:02,500` and `0:01:02.5` give 62.5 s |
| `m:ss[.,]fraction`, minutes can be 60 or more | `01:02.5` gives 62.5 s. `75:00,25` gives 4500.25 s |
| `h:mm:ss:ff` in the clock model, needs `r` | `00:01:02:12` at 25 fps gives 62.48 s |

It fails with an `*InvalidArgumentError` for another shape, for 100000 hours or more, for frames without a rate, and for a frame number at or above the labels per second.
