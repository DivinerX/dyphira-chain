package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

// CLI represents the command-line interface
type CLI struct {
	node *AppNode
	in   io.Reader
	out  io.Writer
}

// NewCLI creates a new CLI instance
func NewCLI(node *AppNode, in io.Reader, out io.Writer) *CLI {
	return &CLI{
		node: node,
		in:   in,
		out:  out,
	}
}

// ternary helper function
func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// Start begins the interactive CLI loop
func (cli *CLI) Start() {
	fmt.Fprintln(cli.out, "=== Dyphira L1 DPoS Blockchain CLI ===")
	fmt.Fprintln(cli.out, "Type 'help' for available commands")
	fmt.Fprintln(cli.out, "Type 'exit' to quit")

	scanner := bufio.NewScanner(cli.in)
	for {
		fmt.Fprint(cli.out, "dyphira> ")
		if !scanner.Scan() {
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		cmd := fields[0]
		args := fields[1:]

		if err := cli.executeCommand(cmd, args); err != nil {
			if err.Error() == "exit" {
				return
			}
			fmt.Fprintf(cli.out, "Error: %v\n", err)
		}
	}
}

// executeCommand handles command execution
func (cli *CLI) executeCommand(cmd string, args []string) error {
	switch cmd {
	case "help":
		return cli.cmdHelp()
	case "exit", "quit":
		return cli.cmdExit()
	case "balance":
		return cli.cmdBalance(args)
	case "account":
		return cli.cmdAccount(args)
	case "send":
		return cli.cmdSend(args)
	case "create":
		return cli.cmdCreate(args)
	case "delegate":
		return cli.cmdDelegate(args)
	case "register":
		return cli.cmdRegister(args)
	case "block":
		return cli.cmdBlock(args)
	case "blocks":
		return cli.cmdBlocks(args)
	case "tx":
		return cli.cmdTx(args)
	case "history":
		return cli.cmdHistory(args)
	case "validators":
		return cli.cmdValidators(args)
	case "peers":
		return cli.cmdPeers(args)
	case "pool":
		return cli.cmdPool(args)
	case "status":
		return cli.cmdStatus(args)
	case "metrics":
		return cli.cmdMetrics(args)
	default:
		fmt.Fprintln(cli.out, "Unknown command. Type 'help' for available commands.")
		return nil
	}
}

// cmdHelp shows available commands
func (cli *CLI) cmdHelp() error {
	fmt.Fprintln(cli.out, "Available commands:")
	fmt.Fprintln(cli.out, "  help  - Show this help message")
	fmt.Fprintln(cli.out, "  exit  - Exit the CLI")
	fmt.Fprintln(cli.out, "  balance [address] - Show balance for address (default: own address)")
	fmt.Fprintln(cli.out, "  account [address] - Show account details for address (default: own address)")
	fmt.Fprintln(cli.out, "  send <to> <amount> [fee] - Send tokens to address")
	fmt.Fprintln(cli.out, "  create <type> <to> <amount> [fee] - Create transaction with type")
	fmt.Fprintln(cli.out, "    Types: transfer, delegate, register_validator")
	fmt.Fprintln(cli.out, "  delegate <validator> <amount> - Delegate stake to validator")
	fmt.Fprintln(cli.out, "  register <stake> - Register as validator")
	fmt.Fprintln(cli.out, "  block <height> - Show block at height")
	fmt.Fprintln(cli.out, "  blocks [start] [end] - Show blocks in range")
	fmt.Fprintln(cli.out, "  tx <hash> - Show transaction by hash")
	fmt.Fprintln(cli.out, "  history <address> - Show transaction history for address")
	fmt.Fprintln(cli.out, "  validators - Show all validators")
	fmt.Fprintln(cli.out, "  peers - Show connected peers")
	fmt.Fprintln(cli.out, "  pool - Show transaction pool")
	fmt.Fprintln(cli.out, "  status - Show node status")
	fmt.Fprintln(cli.out, "  metrics - Show node metrics")
	return nil
}

// cmdExit exits the CLI
func (cli *CLI) cmdExit() error {
	fmt.Fprintln(cli.out, "Goodbye!")
	return fmt.Errorf("exit")
}

// cmdBalance shows balance for an address
func (cli *CLI) cmdBalance(args []string) error {
	var addr Address
	var err error
	if len(args) == 0 {
		addr = cli.node.address
	} else {
		addr, err = HexToAddress(args[0])
		if err != nil {
			return fmt.Errorf("invalid hex address: %v", err)
		}
	}

	acc, err := cli.node.state.GetAccount(addr)
	if err != nil {
		return fmt.Errorf("account not found: %v", err)
	}

	fmt.Fprintf(cli.out, "Address: %s\nBalance: %d\nNonce: %d\n", addr.ToHex(), acc.Balance, acc.Nonce)
	return nil
}

// cmdAccount shows account details
func (cli *CLI) cmdAccount(args []string) error {
	var addr Address
	var err error
	if len(args) == 0 {
		addr = cli.node.address
	} else {
		addr, err = HexToAddress(args[0])
		if err != nil {
			return fmt.Errorf("invalid hex address: %v", err)
		}
	}

	acc, err := cli.node.state.GetAccount(addr)
	if err != nil {
		return fmt.Errorf("account not found: %v", err)
	}

	fmt.Fprintln(cli.out, "Account Details:")
	fmt.Fprintf(cli.out, "  Address: %s\n", addr.ToHex())
	fmt.Fprintf(cli.out, "  Balance: %d\n", acc.Balance)
	fmt.Fprintf(cli.out, "  Nonce: %d\n", acc.Nonce)

	// Try to get validator info
	validator, err := cli.node.vr.GetValidator(addr)
	if err == nil && validator != nil {
		fmt.Fprintf(cli.out, "  Validator Status: %s\n", ternary(validator.Participating, "Active", "Inactive"))
		fmt.Fprintf(cli.out, "  Stake: %d\n", validator.Stake)
		fmt.Fprintf(cli.out, "  Delegated Stake: %d\n", validator.DelegatedStake)
		fmt.Fprintf(cli.out, "  Compute Reputation: %d\n", validator.ComputeReputation)
		fmt.Fprintf(cli.out, "  Participating: %v\n", validator.Participating)
	}

	return nil
}

// cmdSend sends tokens to an address
func (cli *CLI) cmdSend(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: send <to> <amount> [fee]")
	}

	toAddr, err := HexToAddress(args[0])
	if err != nil {
		return fmt.Errorf("invalid recipient address: %v", err)
	}

	var value uint64
	if _, err := fmt.Sscanf(args[1], "%d", &value); err != nil {
		return fmt.Errorf("invalid amount: %v", err)
	}

	if value == 0 {
		return fmt.Errorf("amount must be greater than 0")
	}

	// Parse optional fee
	fee := uint64(1) // Default fee
	if len(args) >= 3 {
		if _, err := fmt.Sscanf(args[2], "%d", &fee); err != nil {
			return fmt.Errorf("invalid fee: %v", err)
		}
	}

	// Get current account and validate balance
	acc, err := cli.node.state.GetAccount(cli.node.address)
	if err != nil {
		return fmt.Errorf("failed to get account: %v", err)
	}

	totalRequired := value + fee
	if acc.Balance < totalRequired {
		return fmt.Errorf("insufficient balance. Required: %d (amount: %d + fee: %d), Available: %d",
			totalRequired, value, fee, acc.Balance)
	}

	// Validate recipient is not the same as sender
	if toAddr == cli.node.address {
		return fmt.Errorf("cannot send to yourself")
	}

	// Show transaction summary before sending
	fmt.Fprintf(cli.out, "Transaction Summary:\n")
	fmt.Fprintf(cli.out, "  From: %s\n", cli.node.address.ToHex())
	fmt.Fprintf(cli.out, "  To: %s\n", toAddr.ToHex())
	fmt.Fprintf(cli.out, "  Amount: %d\n", value)
	fmt.Fprintf(cli.out, "  Fee: %d\n", fee)
	fmt.Fprintf(cli.out, "  Total: %d\n", totalRequired)
	fmt.Fprintf(cli.out, "  Current Balance: %d\n", acc.Balance)
	fmt.Fprintf(cli.out, "  Balance After: %d\n", acc.Balance-totalRequired)
	fmt.Fprintf(cli.out, "  Nonce: %d\n", acc.Nonce+1)

	tx := &Transaction{
		From:      cli.node.address,
		To:        toAddr,
		Value:     value,
		Nonce:     acc.Nonce + 1,
		Fee:       fee,
		Timestamp: time.Now().UnixNano(),
		Type:      "transfer",
	}

	// Sign the transaction
	if err := tx.Sign(cli.node.privKey); err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Broadcast the transaction
	if err := cli.node.BroadcastTransaction(tx); err != nil {
		return fmt.Errorf("failed to broadcast transaction: %v", err)
	}

	fmt.Fprintf(cli.out, "\n✅ Transaction submitted successfully!\n")
	fmt.Fprintf(cli.out, "  Hash: %s\n", tx.Hash.ToHex())
	fmt.Fprintf(cli.out, "  Type: %s\n", tx.Type)
	fmt.Fprintf(cli.out, "  Timestamp: %d\n", tx.Timestamp)

	return nil
}

// cmdCreate creates a transaction with a specific type
func (cli *CLI) cmdCreate(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: create <type> <to> <amount> [fee]")
	}

	txType := args[0]
	toStr := args[1]

	var value uint64
	if _, err := fmt.Sscanf(args[2], "%d", &value); err != nil {
		return fmt.Errorf("invalid amount: %v", err)
	}

	if value == 0 {
		return fmt.Errorf("amount must be greater than 0")
	}

	// Parse optional fee
	fee := uint64(1) // Default fee
	if len(args) >= 4 {
		if _, err := fmt.Sscanf(args[3], "%d", &fee); err != nil {
			return fmt.Errorf("invalid fee: %v", err)
		}
	}

	// Validate transaction type
	if txType != "transfer" && txType != "delegate" && txType != "register_validator" {
		return fmt.Errorf("invalid transaction type. Must be one of: transfer, delegate, register_validator")
	}

	// Parse recipient address
	var toAddr Address
	var err error
	if txType == "register_validator" {
		// For validator registration, use own address
		toAddr = cli.node.address
	} else {
		toAddr, err = HexToAddress(toStr)
		if err != nil {
			return fmt.Errorf("invalid recipient address: %v", err)
		}
	}

	// Get current account and validate balance
	acc, err := cli.node.state.GetAccount(cli.node.address)
	if err != nil {
		return fmt.Errorf("failed to get account: %v", err)
	}

	totalRequired := value + fee
	if acc.Balance < totalRequired {
		return fmt.Errorf("insufficient balance. Required: %d (amount: %d + fee: %d), Available: %d",
			totalRequired, value, fee, acc.Balance)
	}

	// Show transaction summary before sending
	fmt.Fprintf(cli.out, "Transaction Summary:\n")
	fmt.Fprintf(cli.out, "  Type: %s\n", txType)
	fmt.Fprintf(cli.out, "  From: %s\n", cli.node.address.ToHex())
	fmt.Fprintf(cli.out, "  To: %s\n", toAddr.ToHex())
	fmt.Fprintf(cli.out, "  Amount: %d\n", value)
	fmt.Fprintf(cli.out, "  Fee: %d\n", fee)
	fmt.Fprintf(cli.out, "  Total: %d\n", totalRequired)
	fmt.Fprintf(cli.out, "  Current Balance: %d\n", acc.Balance)
	fmt.Fprintf(cli.out, "  Balance After: %d\n", acc.Balance-totalRequired)
	fmt.Fprintf(cli.out, "  Nonce: %d\n", acc.Nonce+1)

	tx := &Transaction{
		From:      cli.node.address,
		To:        toAddr,
		Value:     value,
		Nonce:     acc.Nonce + 1,
		Fee:       fee,
		Timestamp: time.Now().UnixNano(),
		Type:      txType,
	}

	// Sign the transaction
	if err := tx.Sign(cli.node.privKey); err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Broadcast the transaction
	if err := cli.node.BroadcastTransaction(tx); err != nil {
		return fmt.Errorf("failed to broadcast transaction: %v", err)
	}

	fmt.Fprintf(cli.out, "\n✅ Transaction created and submitted successfully!\n")
	fmt.Fprintf(cli.out, "  Hash: %s\n", tx.Hash.ToHex())
	fmt.Fprintf(cli.out, "  Type: %s\n", tx.Type)
	fmt.Fprintf(cli.out, "  Timestamp: %d\n", tx.Timestamp)

	return nil
}

// cmdDelegate delegates stake to a validator
func (cli *CLI) cmdDelegate(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: delegate <validator> <amount>")
	}

	validatorAddr, err := HexToAddress(args[0])
	if err != nil {
		return fmt.Errorf("invalid validator address: %v", err)
	}

	var value uint64
	if _, err := fmt.Sscanf(args[1], "%d", &value); err != nil {
		return fmt.Errorf("invalid amount: %v", err)
	}

	// Get current nonce
	acc, err := cli.node.state.GetAccount(cli.node.address)
	if err != nil {
		return fmt.Errorf("failed to get account: %v", err)
	}

	tx := &Transaction{
		From:      cli.node.address,
		To:        validatorAddr,
		Value:     value,
		Nonce:     acc.Nonce + 1,
		Fee:       1,
		Timestamp: time.Now().UnixNano(),
		Type:      "delegate",
	}

	// Sign the transaction
	if err := tx.Sign(cli.node.privKey); err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Broadcast the transaction
	if err := cli.node.BroadcastTransaction(tx); err != nil {
		return fmt.Errorf("failed to broadcast transaction: %v", err)
	}

	fmt.Fprintf(cli.out, "Delegation submitted: %s\n", tx.Hash.ToHex())
	return nil
}

// cmdRegister registers as a validator
func (cli *CLI) cmdRegister(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: register <stake>")
	}

	var stake uint64
	if _, err := fmt.Sscanf(args[0], "%d", &stake); err != nil {
		return fmt.Errorf("invalid stake amount: %v", err)
	}

	// Get current nonce
	acc, err := cli.node.state.GetAccount(cli.node.address)
	if err != nil {
		return fmt.Errorf("failed to get account: %v", err)
	}

	tx := &Transaction{
		From:      cli.node.address,
		To:        cli.node.address,
		Value:     stake,
		Nonce:     acc.Nonce + 1,
		Fee:       1,
		Timestamp: time.Now().UnixNano(),
		Type:      "register_validator",
	}

	// Sign the transaction
	if err := tx.Sign(cli.node.privKey); err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Broadcast the transaction
	if err := cli.node.BroadcastTransaction(tx); err != nil {
		return fmt.Errorf("failed to broadcast transaction: %v", err)
	}

	fmt.Fprintf(cli.out, "Validator registration submitted: %s\n", tx.Hash.ToHex())
	return nil
}

// cmdBlock shows a specific block
func (cli *CLI) cmdBlock(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: block <height>")
	}

	var height uint64
	if _, err := fmt.Sscanf(args[0], "%d", &height); err != nil {
		return fmt.Errorf("invalid height: %v", err)
	}

	block, err := cli.node.bc.GetBlockByHeight(height)
	if err != nil {
		return fmt.Errorf("block not found: %v", err)
	}

	fmt.Fprintf(cli.out, "Block %d:\n", height)
	fmt.Fprintf(cli.out, "  Hash: %s\n", block.Header.Hash.ToHex())
	fmt.Fprintf(cli.out, "  Previous Hash: %s\n", block.Header.PreviousHash.ToHex())
	fmt.Fprintf(cli.out, "  Timestamp: %d\n", block.Header.Timestamp)
	fmt.Fprintf(cli.out, "  Transactions: %d\n", len(block.Transactions))
	fmt.Fprintf(cli.out, "  Validator: %s\n", block.Header.Proposer.ToHex())

	return nil
}

// cmdBlocks shows blocks in a range
func (cli *CLI) cmdBlocks(args []string) error {
	start := uint64(0)
	end := cli.node.bc.Height()

	if len(args) >= 1 {
		if _, err := fmt.Sscanf(args[0], "%d", &start); err != nil {
			return fmt.Errorf("invalid start height: %v", err)
		}
	}

	if len(args) >= 2 {
		if _, err := fmt.Sscanf(args[1], "%d", &end); err != nil {
			return fmt.Errorf("invalid end height: %v", err)
		}
	}

	fmt.Fprintf(cli.out, "Blocks %d to %d:\n", start, end)
	for h := start; h <= end; h++ {
		block, err := cli.node.bc.GetBlockByHeight(h)
		if err != nil {
			fmt.Fprintf(cli.out, "  %d: Not found\n", h)
			continue
		}
		fmt.Fprintf(cli.out, "  %d: %s (%d txs)\n", h, block.Header.Hash.ToHex()[:16], len(block.Transactions))
	}

	return nil
}

// cmdTx shows a transaction by hash
func (cli *CLI) cmdTx(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tx <hash>")
	}

	hash, err := HexToHash(args[0])
	if err != nil {
		return fmt.Errorf("invalid hash: %v", err)
	}

	for h := cli.node.bc.Height(); h > 0 && h > cli.node.bc.Height()-100; h-- {
		block, err := cli.node.bc.GetBlockByHeight(h)
		if err != nil {
			continue
		}

		for _, tx := range block.Transactions {
			if tx.Hash == hash {
				fmt.Fprintf(cli.out, "Transaction %s:\n", hash.ToHex())
				fmt.Fprintf(cli.out, "  From: %s\n", tx.From.ToHex())
				fmt.Fprintf(cli.out, "  To: %s\n", tx.To.ToHex())
				fmt.Fprintf(cli.out, "  Value: %d\n", tx.Value)
				fmt.Fprintf(cli.out, "  Nonce: %d\n", tx.Nonce)
				fmt.Fprintf(cli.out, "  Type: %s\n", tx.Type)
				fmt.Fprintf(cli.out, "  Block: %d\n", h)
				return nil
			}
		}
	}

	return fmt.Errorf("transaction not found")
}

// cmdHistory shows transaction history for an address
func (cli *CLI) cmdHistory(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: history <address>")
	}

	addr, err := HexToAddress(args[0])
	if err != nil {
		return fmt.Errorf("invalid address: %v", err)
	}

	fmt.Fprintf(cli.out, "Transaction history for %s:\n", addr.ToHex())

	// Search through recent blocks for transactions
	currentHeight := cli.node.bc.Height()
	startHeight := uint64(0)
	if currentHeight > 100 {
		startHeight = currentHeight - 100 // Look at last 100 blocks
	}

	found := false
	for h := currentHeight; h >= startHeight; h-- {
		block, err := cli.node.bc.GetBlockByHeight(h)
		if err != nil {
			continue
		}

		for _, tx := range block.Transactions {
			if tx.From == addr || tx.To == addr {
				if !found {
					found = true
				}

				direction := "OUT"
				if tx.To == addr {
					direction = "IN"
				}

				fmt.Fprintf(cli.out, "  Block %d | %s | Hash: %s\n", h, direction, tx.Hash.ToHex())
				fmt.Fprintf(cli.out, "    From: %s\n", tx.From.ToHex())
				fmt.Fprintf(cli.out, "    To: %s\n", tx.To.ToHex())
				fmt.Fprintf(cli.out, "    Value: %d | Fee: %d | Type: %s\n", tx.Value, tx.Fee, tx.Type)
				fmt.Fprintf(cli.out, "    Nonce: %d | Timestamp: %d\n", tx.Nonce, tx.Timestamp)
				fmt.Fprintf(cli.out, "\n")
			}
		}
	}

	if !found {
		fmt.Fprintf(cli.out, "  No transactions found for this address in the last 100 blocks.\n")
	}

	return nil
}

// cmdValidators shows all validators
func (cli *CLI) cmdValidators(args []string) error {
	validators, err := cli.node.vr.GetAllValidators()
	if err != nil {
		return fmt.Errorf("failed to get validators: %v", err)
	}

	fmt.Fprintf(cli.out, "Validators (%d total):\n", len(validators))
	for _, v := range validators {
		status := ternary(v.Participating, "Active", "Inactive")
		fmt.Fprintf(cli.out, "  %s: %s (stake: %d, delegated: %d)\n",
			v.Address.ToHex()[:16], status, v.Stake, v.DelegatedStake)
	}

	return nil
}

// cmdPeers shows connected peers
func (cli *CLI) cmdPeers(args []string) error {
	peers := cli.node.p2p.host.Network().Peers()

	fmt.Fprintf(cli.out, "Connected peers (%d):\n", len(peers))
	for _, peer := range peers {
		fmt.Fprintf(cli.out, "  %s\n", peer.String())
	}

	return nil
}

// cmdPool shows transaction pool
func (cli *CLI) cmdPool(args []string) error {
	fmt.Fprintln(cli.out, "Transaction pool:")
	fmt.Fprintln(cli.out, "  (Not implemented yet)")
	return nil
}

// cmdStatus shows node status
func (cli *CLI) cmdStatus(args []string) error {
	fmt.Fprintf(cli.out, "Node Status:\n")
	fmt.Fprintf(cli.out, "  Address: %s\n", cli.node.address.ToHex())
	fmt.Fprintf(cli.out, "  Blockchain Height: %d\n", cli.node.bc.Height())
	fmt.Fprintf(cli.out, "  Connected Peers: %d\n", len(cli.node.p2p.host.Network().Peers()))

	validators, _ := cli.node.vr.GetAllValidators()
	fmt.Fprintf(cli.out, "  Validators: %d\n", len(validators))

	return nil
}

// cmdMetrics shows node metrics
func (cli *CLI) cmdMetrics(args []string) error {
	fmt.Fprintln(cli.out, "Node Metrics:")
	fmt.Fprintln(cli.out, "  (Not implemented yet)")
	return nil
}
