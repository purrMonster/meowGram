package fcm

import (
	"context"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"fmt"
	"meowgram/server/internal/repository"
	"testing"
)

type fakeDevices struct{ count int }

func (d fakeDevices) ActiveAfter(_ context.Context, after int64) ([]repository.PushDevice, error) {
	devices := []repository.PushDevice{}
	for id := after + 1; id <= int64(d.count) && len(devices) < 500; id++ {
		devices = append(devices, repository.PushDevice{ID: id, Token: fmt.Sprintf("scratch-%d", id)})
	}
	return devices, nil
}
func (d fakeDevices) DeleteInvalid(context.Context, string) error { return nil }

type fakeSender struct {
	calls int
	total int
	fail  bool
}

func (f *fakeSender) SendEachForMulticast(_ context.Context, msg *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	f.calls++
	f.total += len(msg.Tokens)
	if f.fail {
		return nil, errors.New("provider error contains sensitive token")
	}
	result := &messaging.BatchResponse{}
	for range msg.Tokens {
		result.Responses = append(result.Responses, &messaging.SendResponse{Success: true})
	}
	return result, nil
}
func TestPushUsesOnlyRegisteredDevicesInBatches(t *testing.T) {
	for _, count := range []int{0, 1, 1001} {
		sender := &fakeSender{}
		service := Service{client: sender, devices: fakeDevices{count}}
		if err := service.PublishActivity(context.Background(), "title", "content-free", map[string]string{"type": "lounge_activity"}); err != nil {
			t.Fatal(err)
		}
		if sender.total != count || sender.calls != (count+499)/500 {
			t.Fatalf("incorrect recipient count: %+v", sender)
		}
	}
}
func TestPushErrorsDoNotExposeTokens(t *testing.T) {
	service := Service{client: &fakeSender{fail: true}, devices: fakeDevices{1}}
	err := service.PublishActivity(context.Background(), "title", "content-free", nil)
	if err == nil || err.Error() != "push provider request failed" {
		t.Fatal("unsafe provider error")
	}
}
