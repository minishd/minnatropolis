package room

import (
	"context"
	"log"
	"time"
)

// Send a packet to everyone in every room.
func (h *Handler) broadcast(msgs ...any) {
	h.usersMu.RLock()
	defer h.usersMu.RUnlock()
	for _, user := range h.users {
		user.Send(msgs...)
	}
	// This is an OK usecase for [gws.Broadcaster]
	// In the future, see if it's faster than queuing maybe
}

func getUntil59() time.Duration {
	now := time.Now()
	secondsUntil59 := 59 - now.Second()
	if secondsUntil59 <= 0 {
		// if we already passed the 59th second,
		// wait until next minute..
		secondsUntil59 += 60
	}
	return time.Duration(secondsUntil59) * time.Second
}

func (h *Handler) Background(ctx context.Context) {
	if h.coun == nil {
		// We don't need to do anything
		// for games that aren't Collective Unconscious
		return
	}

	// Wait for next minute to begin time/weather updates..
	// (we actually want to trigger right before the minute
	// to give update packets a moment to send)
	until59 := getUntil59()
	timeTimer := time.NewTicker(until59)
	weatherTimer := time.NewTicker(until59)
	var didResetTime, didResetWeather bool
	defer timeTimer.Stop()
	defer weatherTimer.Stop()

	// Wait for tick or cancel
	for {
		select {
		case <-timeTimer.C:
			if !didResetTime {
				timeTimer.Reset(time.Minute)
				didResetTime = true
			}
			h.coun.OnTimeTick()
			h.broadcast(h.coun.GetTimePacket())
		case <-weatherTimer.C:
			if !didResetWeather {
				weatherTimer.Reset(time.Minute * 2)
				didResetWeather = true
			}
			h.coun.OnWeatherTick()
			h.broadcast(h.coun.GetWeatherPacket())
		case <-ctx.Done():
			log.Println("handler bg task ended!")
			return
		}
	}
}
