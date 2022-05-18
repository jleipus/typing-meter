package main

import (
	"flag"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"typingMeter/mapsort"
	"typingMeter/timectrl"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
)

const keyPollingDelay time.Duration = 10

var wg = sync.WaitGroup{}

func main() {
	interval := flag.Int("interval", 5, "time in seconds between intervals")
	limit := flag.Int("limit", 20, "time limit in seconds")
	flag.Parse()

	fmt.Println("Typing Meter")
	fmt.Println("---------------------")

	wg.Add(3)

	tc := timectrl.NewTimeController(timectrl.WithTicker(*interval), timectrl.WithTimer(*limit))

	go tc.RunTicker(&wg)
	go tc.RunTimer(&wg)
	go readInput(tc, *interval)

	wg.Wait()
}

func readInput(tc *timectrl.TimeController, interval int) {
	var keys []byte
	var index int

	var lastKey int
	var activeKey int

	isFinished := false
	for !isFinished {
		activeKey = readKey() // Gets value of pressed key

		if activeKey != 0 { // If a key is pressed
			if activeKey != lastKey { // If not holding down same key
				lastKey = activeKey
				keys = append(keys, byte(activeKey))

				startIndex := func(len int) int {
					if len-5 < 0 {
						return 0
					}
					return len - 5
				}(len(keys))

				input := string(keys[startIndex:]) // Converting last 5 characters to one string

				switch {
				case strings.Contains(input, "START"):
					fmt.Println("\nCommand received: START")
					fmt.Println("Switching from timed mode to manual mode")
					fmt.Println("Type END to stop meter")

					// Resetting all variables to initial values
					tc.StopAll()
					tc = timectrl.NewTimeController(timectrl.WithTicker(interval))

					wg.Add(1)
					go tc.RunTicker(&wg)

					keys = nil
					index = 0
				case strings.Contains(input, "END"):
					if !tc.TimedMode {
						fmt.Println("\nCommand received: END")
						tc.StopTicker()
						isFinished = true
					}
				}
			}
		} else {
			lastKey = 0
		}

		select {
		case <-tc.IntervalCh:
			wg.Add(1)
			fmt.Println("\nInterval statistics:")
			go calculateStats(keys[index:], len(keys), tc) // Sending slice with only the keys pressed during last interval

			index = len(keys) // Saving new index for next interval
		case <-tc.DoneCh:
			tc.StopTicker()
			isFinished = true
		default:
			time.Sleep(keyPollingDelay * time.Millisecond)
		}
	}

	wg.Add(1)
	fmt.Println("\nSession complete, overall statistics:")
	go calculateStats(keys, len(keys), tc)

	wg.Done()
}

func readKey() (activeKey int) {
	for i := 0; i < 256; i++ {
		keyState, _, _ := procGetAsyncKeyState.Call(uintptr(i)) // Checking keystate for all keys in range (0, 255)

		if keyState&(1<<15) != 0 && !(i < 0x2F && i != 0x20) && (i < 160 || i > 165) && (i < 91 || i > 93) { // If key is pressed and satisfies conditions
			activeKey = i
			break
		} else {
			activeKey = 0
		}
	}

	return activeKey
}

func calculateStats(keys []byte, totalKeyCount int, tc *timectrl.TimeController) {
	timeElapsed := tc.TimePassed() // Receiving time since session start

	total := len(keys)
	speed := float64(totalKeyCount) / timeElapsed.Seconds()

	popularityMap := sliceToMap(keys)
	sorted := mapsort.SortMapByValue(popularityMap)

	fmt.Printf("Keys pressed: %v\n", total)
	fmt.Printf("Typing speed: %.2f\n", speed)

	fmt.Println("Most pressed keys:")
	for i := 0; i < 3; i++ {
		if i < sorted.Len() {
			fmt.Printf("%v. \"%v\": %v\n", i+1, sorted[i].Key, sorted[i].Value)
		}
	}

	wg.Done()
}

func sliceToMap(s []byte) map[string]int {
	ret := make(map[string]int)
	for _, key := range s {
		ret[string(key)]++
	}

	return ret
}
