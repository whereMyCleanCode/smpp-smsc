package smsc

import "github.com/bwmarrin/snowflake"

type IDGenerator interface {
	GenerateID() (uint64, error)
}

type SnowflakeGenerator struct {
	node *snowflake.Node
}

func NewSnowflakeGenerator(nodeID int64) (*SnowflakeGenerator, error) {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil, err
	}
	return &SnowflakeGenerator{node: node}, nil
}

func (g *SnowflakeGenerator) GenerateID() (uint64, error) {
	id := g.node.Generate()
	return uint64(id.Int64()), nil
}

var _ IDGenerator = (*SnowflakeGenerator)(nil)
