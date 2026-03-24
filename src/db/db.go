package db

import (
	"context"
	"strings"

	"github.com/redis/rueidis"
	"github.com/rs/zerolog/log"
)

type Client struct {
	Client rueidis.Client
}

func NewClient(addr string) (*Client, error) {
	var (
		client rueidis.Client
		err    error
	)

	client, err = rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{addr},
	})
	switch {
	case err != nil:
		log.Error().Err(err).Str("addr", addr).Msg("Failed to create Redis client")
		return nil, err
	}

	return &Client{Client: client}, nil
}

func (c *Client) Close() {
	c.Client.Close()
}

func (c *Client) Set(ctx context.Context, key string, val any) error {
	var (
		err error
		res rueidis.RedisResult
	)

	res = c.Client.Do(ctx, c.Client.B().JsonSet().Key(key).Path("$").Value(rueidis.JSON(val)).Build())
	err = res.Error()
	switch {
	case err != nil:
		log.Error().Err(err).Str("key", key).Msg("Failed to set JSON key in Redis")
	}

	return err
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	var (
		err error
		res rueidis.RedisResult
		val string
	)

	res = c.Client.Do(ctx, c.Client.B().JsonGet().Key(key).Build())
	err = res.Error()
	switch {
	case err != nil:
		log.Error().Err(err).Str("key", key).Msg("Failed to get JSON key from Redis")
		return "", err
	}

	val, err = res.ToString()
	return val, err
}

func (c *Client) CreateIndex(ctx context.Context, index string) error {
	var (
		err error
		res rueidis.RedisResult
	)

	// FT.CREATE idx:dn ON JSON PREFIX 1 dn: SCHEMA $.dn AS dn TAG $.objectClass[*] AS objectClass TAG $.attributes.cn[*] AS cn TEXT
	res = c.Client.Do(ctx, c.Client.B().FtCreate().Index(index).
		OnJson().
		Prefix(1).Prefix("dn:").
		Schema().
		FieldName("$.dn").As("dn").Tag().
		FieldName("$.objectClass[*]").As("objectClass").Tag().
		FieldName("$.attributes.cn[*]").As("cn").Text().
		Build())
	err = res.Error()
	switch {
	case err != nil && !strings.Contains(err.Error(), "Index already exists"):
		log.Error().Err(err).Str("index", index).Msg("Failed to create index")
		return err
	}

	return nil
}

func (c *Client) Search(ctx context.Context, index string, query string) ([]string, error) {
	var (
		err   error
		res   rueidis.RedisResult
		val   []string
		total int64
		docs  []rueidis.FtSearchDoc
	)

	res = c.Client.Do(ctx, c.Client.B().FtSearch().Index(index).Query(query).Build())
	err = res.Error()
	switch {
	case err != nil:
		log.Error().Err(err).Str("index", index).Str("query", query).Msg("Failed to search in Redis")
		return nil, err
	}

	total, docs, err = res.AsFtSearch()
	switch {
	case err != nil:
		return nil, err
	}

	for _, doc := range docs {
		var (
			jsonStr string
			ok      bool
		)
		// When using RediSearch with RedisJSON, the field '$' contains the full JSON if requested or available
		jsonStr, ok = doc.Doc["$"]
		switch {
		case ok:
			val = append(val, jsonStr)
		default:
			// Fallback or handle differently
		}
	}

	_ = total
	return val, nil
}
