package common

type ProtocolID string

const (
	ProtocolPrefix = "/dogechain"

	DiscProto     ProtocolID = ProtocolPrefix + "/disc/0.1"
	IdentityProto ProtocolID = ProtocolPrefix + "/id/0.1"
	SyncerV1Proto ProtocolID = ProtocolPrefix + "/syncer/0.1"
)
