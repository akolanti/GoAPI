package store

import (
	"context"
	"sync"

	"github.com/akolanti/GoAPI/internal/domain/jobModel"
)

type InMemoryMessageStore struct {
	chatLock *sync.RWMutex
	chatMap  map[string][]jobModel.JobPayload
}

func InitMessageStore() *InMemoryMessageStore {
	return &InMemoryMessageStore{
		chatLock: new(sync.RWMutex),
		chatMap:  make(map[string][]jobModel.JobPayload),
	}
}

func (store *InMemoryMessageStore) ValidateChatId(ctx context.Context, chatId string) bool {
	store.chatLock.RLock()
	defer store.chatLock.RUnlock()
	_, ok := store.chatMap[chatId]
	return ok
}

func (store *InMemoryMessageStore) saveChatId(id string, conversation jobModel.JobPayload) {
	store.chatLock.Lock()
	defer store.chatLock.Unlock()
	store.chatMap[id] = append(store.chatMap[id], conversation)
	inMemLogger.Info(id, " : Saved convo to chat message store")
}

func (store *InMemoryMessageStore) TrySaveChat(ctx context.Context, id string, conversation jobModel.JobPayload) error {
	if store.ValidateChatId(ctx, id) == false {
		return nil
	}
	store.saveChatId(id, conversation)
	return nil
}

func (store *InMemoryMessageStore) InitNewChat(ctx context.Context, id string) error {
	store.chatLock.Lock()
	defer store.chatLock.Unlock()
	store.chatMap[id] = make([]jobModel.JobPayload, 0)
	return nil
}

func (store *InMemoryMessageStore) GetMessageHistory(ctx context.Context, chatId string) (error, []string) {

	return nil, nil
}
