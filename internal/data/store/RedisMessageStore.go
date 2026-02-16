package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/data/redisStore"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

type RedisMessageStore struct {
	store  *redisStore.Store
	logger *logger_i.Logger
}

func GetRedisMessageStore(ctx context.Context) *RedisMessageStore {
	return &RedisMessageStore{
		store:  redisStore.GetRedisStore(ctx, config.RedisMessageStore),
		logger: logger_i.NewLogger("MessageStore"),
	}
}

func (s *RedisMessageStore) ValidateChatId(ctx context.Context, chatId string) bool {
	log := s.logger.With("traceId", ctx.Value(config.TRACE_ID_KEY), "chat Id", chatId)
	log.Debug("validating chatId")
	isFound, err := s.store.Exists(ctx, chatId)
	if s.store.IsNil(err) {
		return false
	} else if err != nil {
		log.Error("Failed to check if chatId exists", "err", err)
		return false
	}
	return isFound
}

func (s *RedisMessageStore) TrySaveChat(ctx context.Context, id string, conversation jobModel.JobPayload) error {
	log := s.logger.With("traceId", ctx.Value(config.TRACE_ID_KEY), "chat Id", id)
	if s.ValidateChatId(ctx, id) == false {
		err := errors.New("invalid chat id")
		log.Error("Failed Validation before saving", "err", err)
		return err
	}
	return s.saveChatId(ctx, id, conversation)
}

func (s *RedisMessageStore) saveChatId(ctx context.Context, id string, conversation jobModel.JobPayload) error {
	log := s.logger.With("traceId", ctx.Value(config.TRACE_ID_KEY), "chat Id", id)
	err := s.store.ListPush(ctx, id, marshallJson(conversation, s.logger))
	if err != nil {
		log.Error("error saving chat", "error:", err)
	}
	log.Debug("Saved chat successfully")
	return err
}

func (s *RedisMessageStore) InitNewChat(ctx context.Context, id string) error {
	log := s.logger.With("traceId", ctx.Value(config.TRACE_ID_KEY).(string), "chat Id", id)
	log.Debug("Initializing new chat")
	err := s.store.Del(ctx, id)
	if s.store.IsNil(err) {
		log.Error("Error initializing chat", id)
	}
	return s.saveChatId(ctx, id, jobModel.JobPayload{})
}

func marshallJson(payload jobModel.JobPayload, logger *logger_i.Logger) []byte {
	data, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Error marshalling json :", err)
	}
	return data
}

func (s *RedisMessageStore) GetMessageHistory(ctx context.Context, chatId string) (error, []string) {
	log := s.logger.With("traceId", ctx.Value(config.TRACE_ID_KEY).(string), "chat Id", chatId)
	log.Debug("Getting message history")

	res, err := s.store.ListGet5PastMessage(ctx, chatId)

	if err != nil {
		log.Error("Error getting history", "error:", err)
		return err, nil
	}
	return nil, utils.ReverseStringArray(res)
}
