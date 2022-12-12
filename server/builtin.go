package server

import (
	"github.com/dogechain-lab/dogechain/consensus"
	consensusDev "github.com/dogechain-lab/dogechain/consensus/dev"
	consensusDummy "github.com/dogechain-lab/dogechain/consensus/dummy"
	consensusIBFT "github.com/dogechain-lab/dogechain/consensus/ibft"
	"github.com/dogechain-lab/dogechain/secrets"
	"github.com/dogechain-lab/dogechain/secrets/awsssm"
	"github.com/dogechain-lab/dogechain/secrets/hashicorpvault"
	"github.com/dogechain-lab/dogechain/secrets/local"
)

type ConsensusType string

const (
	DevConsensus   ConsensusType = "dev"
	IBFTConsensus  ConsensusType = "ibft"
	DummyConsensus ConsensusType = "dummy"
)

var consensusBackends = map[ConsensusType]consensus.Factory{
	DevConsensus:   consensusDev.Factory,
	IBFTConsensus:  consensusIBFT.Factory,
	DummyConsensus: consensusDummy.Factory,
}

// secretsManagerBackends defines the SecretManager factories for different
// secret management solutions
var secretsManagerBackends = map[secrets.SecretsManagerType]secrets.SecretsManagerFactory{
	secrets.Local:          local.SecretsManagerFactory,
	secrets.HashicorpVault: hashicorpvault.SecretsManagerFactory,
	secrets.AWSSSM:         awsssm.SecretsManagerFactory,
}

func ConsensusSupported(value string) bool {
	_, ok := consensusBackends[ConsensusType(value)]

	return ok
}

func GetConsensusBackend(value string) (consensus.Factory, bool) {
	consensus, ok := consensusBackends[ConsensusType(value)]

	return consensus, ok
}

func GetSecretsManager(secretType secrets.SecretsManagerType) (secrets.SecretsManagerFactory, bool) {
	secretManager, ok := secretsManagerBackends[secretType]

	return secretManager, ok
}
