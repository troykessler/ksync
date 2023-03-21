package p2p

import (
	cfg "KYVENetwork/ksync/config"
	p2pHelpers "KYVENetwork/ksync/executor/p2p/helpers"
	"KYVENetwork/ksync/executor/p2p/reactor"
	log "KYVENetwork/ksync/logger"
	"KYVENetwork/ksync/pool"
	"KYVENetwork/ksync/types"
	"fmt"
	"github.com/tendermint/tendermint/crypto/ed25519"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"net/url"
	"os"
	"strconv"
)

var (
	blockCh = make(chan *types.Block, 1000)
	quitCh  = make(chan int)
	logger  = log.Logger()
)

func StartP2PExecutor(homeDir string, poolId int64, restEndpoint string, targetHeight int64) {
	// load config
	config, err := cfg.LoadConfig(homeDir)
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	// load start and latest height
	startHeight, latestHeight := pool.GetPoolInfo(restEndpoint, poolId)

	// if target height is smaller than the base height of the pool we exit
	if targetHeight > 0 && targetHeight < startHeight {
		logger.Error(fmt.Sprintf("target height %d is smaller than pool starting height %d", targetHeight, startHeight))
		os.Exit(1)
	}

	// if the latest height of the pool is smaller than the target height we decrease the target height to this value
	if latestHeight < targetHeight {
		targetHeight = latestHeight
	}

	peerAddress := config.P2P.ListenAddress
	peerHost, err := url.Parse(peerAddress)
	if err != nil {
		panic(fmt.Errorf("invalid peer address: %w", err))
	}

	port, err := strconv.ParseInt(peerHost.Port(), 10, 64)
	if err != nil {
		panic(fmt.Errorf("invalid peer port: %w", err))
	}

	// this peer should listen to different port to avoid port collision
	config.P2P.ListenAddress = fmt.Sprintf("tcp://%s:%d", peerHost.Hostname(), port-1)

	logger.Info(fmt.Sprintf("Config loaded. Moniker = %s", config.Moniker))

	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		panic(fmt.Errorf("failed to load node key file: %w", err))
	}

	// generate new node key for this peer
	ksyncNodeKey := &p2p.NodeKey{
		PrivKey: ed25519.GenPrivKey(),
	}

	logger.Info(fmt.Sprintf("generated new node key with id = %s", ksyncNodeKey.ID()))

	genDoc, err := nm.DefaultGenesisDocProviderFunc(config)()
	if err != nil {
		panic(fmt.Errorf("failed to load state and genDoc: %w", err))
	}

	nodeInfo, err := p2pHelpers.MakeNodeInfo(config, ksyncNodeKey, genDoc)

	logger.Info("created node info")

	transport := p2p.NewMultiplexTransport(nodeInfo, *ksyncNodeKey, p2p.MConnConfig(config.P2P))

	logger.Info("created multiplex transport")

	p2pLogger := logger.With("module", "p2p")
	bcR := reactor.NewBlockchainReactor(blockCh, quitCh, poolId, restEndpoint, startHeight, latestHeight)
	sw := p2pHelpers.CreateSwitch(config, transport, bcR, nodeInfo, ksyncNodeKey, p2pLogger)

	// start the transport
	addr, err := p2p.NewNetAddressString(p2p.IDAddressString(ksyncNodeKey.ID(), config.P2P.ListenAddress))
	if err != nil {
		panic(fmt.Errorf("failed to start transport: %w", err))
	}
	if err := transport.Listen(*addr); err != nil {
		panic(fmt.Errorf("failed to start transport: %w", err))
	}

	persistentPeers := make([]string, 0)
	peerString := fmt.Sprintf("%s@%s:%s", nodeKey.ID(), peerHost.Hostname(), peerHost.Port())
	persistentPeers = append(persistentPeers, peerString)

	if err := sw.AddPersistentPeers(persistentPeers); err != nil {
		panic("could not add persistent peers")
	}

	// start switch
	err = sw.Start()
	if err != nil {
		panic(fmt.Errorf("failed to start switch: %w", err))
	}

	// get peer
	peer, err := p2p.NewNetAddressString(peerString)
	if err != nil {
		panic(fmt.Errorf("invalid peer address: %w", err))
	}

	if err := sw.DialPeerWithAddress(peer); err != nil {
		logger.Error(fmt.Sprintf("Failed to dial peer %v", err.Error()))
	}
}