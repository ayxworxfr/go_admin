package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pkModel struct {
	ID   uint64 `xorm:"pk autoincr bigint unsigned 'id'"`
	Name string `xorm:"varchar(50) 'name'"`
}

type noPKModel struct {
	Name string `xorm:"varchar(50) 'name'"`
}

func TestMetaOfPrimaryKey(t *testing.T) {
	meta, err := metaOf[pkModel]()
	require.NoError(t, err)
	assert.Equal(t, "id", meta.pkColumn)
	assert.Equal(t, 0, meta.pkFieldIndex)

	// 二次调用走缓存
	meta2, err := metaOf[pkModel]()
	require.NoError(t, err)
	assert.Same(t, meta, meta2)
}

func TestMetaOfNoPrimaryKey(t *testing.T) {
	_, err := metaOf[noPKModel]()
	assert.ErrorIs(t, err, ErrNoPrimaryKey)
}

func TestSetAndReadPrimaryKey(t *testing.T) {
	m := &pkModel{}
	require.NoError(t, setPrimaryKey(m, uint64(42)))
	assert.Equal(t, uint64(42), m.ID)

	v, err := primaryKeyValue(m)
	require.NoError(t, err)
	assert.Equal(t, uint64(42), v)
}
