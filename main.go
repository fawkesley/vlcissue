package main

import (
	"log"
	"math/rand"
	"time"

	vlc "github.com/adrg/libvlc-go/v3"
)

func main() {
	mp := New()

	const duration = 3 * time.Second

	for i := 0; ; i++ {
		log.Printf("video #%d", i)

		video := "video.mp4"

		mp.PlayVideoFor(video, duration, func(remaining time.Duration) {
			log.Printf("callback, remaining: %s", remaining)
		})

		time.Sleep(2 * time.Second)
	}
}

// New returns a new Player
func New() Player {
	p := Player{}
	return p
}

// Player can play videos
type Player struct {
}

// PlayVideoFor plays the given video and returns after `playFor`
func (p *Player) PlayVideoFor(filename string, playFor time.Duration, callback func(time.Duration)) {

	// See `vlc -h` for more command-line options
	if err := vlc.Init("--quiet", "--fullscreen"); err != nil {
		log.Fatal(err)
	}
	defer func() {
		log.Printf("calling vlc.Release()")
		if err := vlc.Release(); err != nil {
			log.Fatalf("vlc.Release() error: %v", err)
		}
	}()

	logf := func(fmt string, args ...interface{}) {
		log.Printf(filename+": "+fmt, args...)
	}

	player, err := vlc.NewPlayer()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		logf("calling player.Stop()")
		if err := player.Stop(); err != nil {
			log.Fatalf("player.Stop() error: %v", err)
		}

		logf("calling player.Release()")
		if err := player.Release(); err != nil {
			log.Fatalf("player.Release() error: %v", err)
		}
	}()

	media, err := player.LoadMediaFromPath(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := media.Release(); err != nil {
			logf("media.Release() error: %v", err)
		}
	}()

	// Retrieve player event manager.
	manager, err := player.EventManager()
	if err != nil {
		log.Fatal(err)
	}

	// Create event handler.
	quit := make(chan struct{})

	var endAt time.Time = time.Now().Add(playFor)

	eventCallback := func(event vlc.Event, userData interface{}) {
		switch event {

		case vlc.MediaPlayerPlaying:

			logf("vlc event: MediaPlayerPlaying")
			fullscreen, err := player.IsFullScreen()
			if err == nil && !fullscreen {
				logf("MediaPlayerPlaying, fullscreen=%t, switching ON", fullscreen)
				player.SetFullScreen(true)
			}

			millis, err := player.MediaLength() // must come after Play
			if err != nil {
				log.Fatal(err)
			}
			logf("media length: %s", time.Duration(millis)*time.Millisecond)

			isSeekable := player.IsSeekable()
			logf("isSeekable: %t", isSeekable)

			maxSeekPosition := 1 - (float32(playFor.Milliseconds()) / float32(millis))
			seekPosition := rand.Float32() * maxSeekPosition
			logf("maxSeekPosition: %f, seekPosition: %f", maxSeekPosition, seekPosition)

			logf("calling SetMediaPosition")
			err = player.SetMediaPosition(seekPosition)
			logf("SetMediaPosition returned %v", err)
			if err != nil {
				log.Fatal(err)
			}

		case vlc.MediaPlayerLengthChanged:
			logf("vlc event: MediaPlayerLengthChanged")

		case vlc.MediaPlayerEndReached:
			logf("vlc event: PlayerEndReached")
			close(quit)

		case vlc.MediaPlayerTimeChanged:
			millis, err := player.MediaTime()
			if err != nil {
				log.Println(err)
				break
			}
			logf("@ %s", time.Duration(millis)*time.Millisecond)

			if callback != nil {
				callback(endAt.Sub(time.Now()))
			}
		}
	} // eventCallback

	// Register events with the event manager.
	events := []vlc.Event{

		vlc.MediaPlayerPlaying,
		vlc.MediaPlayerLengthChanged,
		vlc.MediaPlayerTimeChanged,
		vlc.MediaPlayerEndReached,
	}

	var eventIDs []vlc.EventID
	for _, event := range events {
		eventID, err := manager.Attach(event, eventCallback, nil)
		if err != nil {
			log.Fatal(err)
		}

		eventIDs = append(eventIDs, eventID)
	}

	// De-register attached events.
	defer manager.Detach(eventIDs...)

	media, err = player.Media()
	if err != nil {
		log.Fatal(err)
	}

	logf("calling Play()")
	if err = player.Play(); err != nil {
		log.Fatal(err)
	}

	go func() {
		time.Sleep(endAt.Sub(time.Now()))
		logf("timeout (%s), sending quit", playFor)
		close(quit)
	}()

	<-quit
}
