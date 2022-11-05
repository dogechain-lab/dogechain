package common

type ProtocolId string

const (
	ProtocolPrefix = "/dogechain"

	DiscProto     ProtocolId = ProtocolPrefix + "/disc/0.1"
	IdentityProto ProtocolId = ProtocolPrefix + "/id/0.1"
	SyncerV1Proto ProtocolId = ProtocolPrefix + "/syncer/0.1"
)
