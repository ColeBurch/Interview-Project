package main

import (
	"reflect"
	"testing"
)

func TestJoinAndLeaveBroadcastToOtherViewers(t *testing.T) {
	p := newPresence()

	cole, leaveCole := p.join("weekly", "Cole")
	if got := <-cole.updates; !reflect.DeepEqual(got, []string{"Cole"}) {
		t.Fatalf("after Cole joins: got %v", got)
	}

	sarah, leaveSarah := p.join("weekly", "Sarah")
	defer leaveSarah()
	want := []string{"Cole", "Sarah"}
	if got := <-cole.updates; !reflect.DeepEqual(got, want) {
		t.Fatalf("Cole after Sarah joins: got %v", got)
	}
	if got := <-sarah.updates; !reflect.DeepEqual(got, want) {
		t.Fatalf("Sarah after joining: got %v", got)
	}

	leaveCole()
	if got := <-sarah.updates; !reflect.DeepEqual(got, []string{"Sarah"}) {
		t.Fatalf("Sarah after Cole leaves: got %v", got)
	}
}

func TestReportsAreIsolated(t *testing.T) {
	p := newPresence()

	cole, leaveCole := p.join("weekly", "Cole")
	defer leaveCole()
	<-cole.updates

	_, leaveJake := p.join("monthly", "Jake")
	defer leaveJake()

	select {
	case got := <-cole.updates:
		t.Fatalf("Cole received update from another report: %v", got)
	default:
	}
}

func TestSameNameInTwoTabsListedOnce(t *testing.T) {
	p := newPresence()

	_, leaveA := p.join("weekly", "Cole")
	defer leaveA()
	b, leaveB := p.join("weekly", "Cole")
	defer leaveB()

	if got := <-b.updates; !reflect.DeepEqual(got, []string{"Cole"}) {
		t.Fatalf("got %v", got)
	}
}
