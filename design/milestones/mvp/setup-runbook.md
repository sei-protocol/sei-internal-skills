# MVP Setup Runbook

**Date:** 2026-04-02
**Audience:** Brandon (sole operator)
**Goal:** Get the full Tide MVP agent loop running on Sei arctic-1 testnet, from zero to first on-chain ProposalApproved event.

**Total estimated time:** 2-3 hours, mostly waiting for faucet and deployment.

---

## Section 1: Prerequisites

Everything in this section must be in place before you run any scripts.

### AWS

- AWS account with an IAM user or role that can:
  - `kms:CreateKey`, `kms:CreateAlias`, `kms:GetPublicKey`, `kms:Sign`
  - `secretsmanager:CreateSecret`, `secretsmanager:UpdateSecret`, `secretsmanager:GetSecretValue`
  - `iam:CreateRole`, `iam:AttachRolePolicy`, `iam:CreatePolicy` (for IRSA setup)
  - `eks:DescribeCluster` (to get the OIDC issuer URL for IRSA)
- Configure your CLI profile before starting:
  ```bash
  aws configure  # or export AWS_PROFILE=your-profile
  aws sts get-caller-identity  # verify
  ```
- Choose a region and export it:
  ```bash
  export AWS_REGION=us-east-1  # or us-west-2 -- pick one, use it everywhere
  ```

### GitHub

- Org admin access to `sei-protocol` (needed to create a GitHub App at the org level)
- `gh` CLI v2.40+ installed and authenticated:
  ```bash
  gh auth login  # authenticate against github.com
  gh auth status  # verify
  ```
- The following repos must exist (or you create them in the manual steps below):
  - `sei-protocol/tide-proposals`
  - `sei-protocol/tide-deliverables`
  - `sei-protocol/tide-workspace-blockchain-dev`
  - `sei-protocol/tide-workspace-k8s-specialist`
  - `sei-protocol/tide-workspace-platform-eng`

### Sei arctic-1 Testnet

- Faucet access. Two options:
  - Web faucet: https://arctic-1.sei.io/faucet
  - Sei Discord `#testnet-faucet` channel (faster, no rate limit issues)
- A deployer EOA wallet with private key. Create one with:
  ```bash
  cast wallet new
  # Save the address and private key somewhere secure (1Password, etc.)
  export DEPLOYER_ADDRESS=0x...
  export DEPLOYER_KEY=0x...
  ```
- Fund the deployer wallet with at least 20 SEI testnet tokens.

### Local Toolchain

All must be installed before running scripts:

| Tool | Version | Install |
|------|---------|---------|
| `aws` CLI | v2.x | https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html |
| `gh` CLI | v2.40+ | `brew install gh` |
| Foundry (`forge`, `cast`) | latest | `curl -L https://foundry.paradigm.xyz | bash && foundryup` |
| `kubectl` | v1.28+ | `brew install kubectl` |
| `tkn` (Tekton CLI) | v0.35+ | `brew install tektoncd-cli` |
| `jq` | v1.6+ | `brew install jq` |
| `openssl` | system | Pre-installed on macOS |
| `xxd` | system | Pre-installed on macOS |
| `python3` | v3.12+ | `brew install python@3.12` |

Verify all at once:

```bash
for tool in aws gh forge cast kubectl tkn jq openssl xxd python3; do
    if command -v "$tool" &>/dev/null; then
        echo "OK: $tool $(${tool} --version 2>&1 | head -1)"
    else
        echo "MISSING: $tool"
    fi
done
```

### Kubernetes Cluster

- An EKS cluster (or any K8s 1.28+ cluster with IRSA-equivalent workload identity) with:
  - Tekton Pipelines v0.56+ installed
  - Tekton Triggers v0.26+ installed
  - AWS Secrets Store CSI Driver + AWS Provider installed
  - An OIDC provider configured for IRSA (EKS does this by default)
- `kubectl` context pointing at the right cluster:
  ```bash
  kubectl config current-context  # verify
  kubectl get nodes               # should show Ready nodes
  ```
- Verify Tekton is running:
  ```bash
  kubectl get pods -n tekton-pipelines
  kubectl get pods -n tekton-triggers
  ```

---

## Section 2: Manual Steps (Only Brandon Can Do These)

These steps require a browser or interactive authentication that cannot be scripted.

### Step M1: Create GitHub Repos in sei-protocol

If the Tide repos do not already exist:

```bash
# Create all repos as private initially
gh repo create sei-protocol/tide-proposals --private --description "Tide design proposals and agent reviews"
gh repo create sei-protocol/tide-deliverables --private --description "Tide agent code deliverables"
gh repo create sei-protocol/tide-workspace-blockchain-dev --private --description "Blockchain dev agent workspace"
gh repo create sei-protocol/tide-workspace-k8s-specialist --private --description "K8s specialist agent workspace"
gh repo create sei-protocol/tide-workspace-platform-eng --private --description "Platform engineer agent workspace"

# Initialize with a README so clone works
for repo in tide-proposals tide-deliverables tide-workspace-blockchain-dev tide-workspace-k8s-specialist tide-workspace-platform-eng; do
    gh repo edit sei-protocol/$repo --enable-issues
done
```

### Step M2: Create the GitHub App

1. Open https://github.com/organizations/sei-protocol/settings/apps/new in your browser.

2. Fill in:
   - **App name:** `tide-council-bot`
   - **Description:** `Tide AI agent council — automated design reviews, code execution, and on-chain attestation`
   - **Homepage URL:** `https://github.com/sei-protocol`
   - **Webhook URL:** Leave as `https://placeholder.example.com` for now — you will update it after Tekton is deployed
   - **Webhook secret:** Generate and copy one now:
     ```bash
     openssl rand -hex 32
     # Save this value — you will need it in Step S1 and when setting the webhook
     ```

3. Set permissions (minimum required):
   - Repository > Contents: Read & Write
   - Repository > Pull requests: Read & Write
   - Repository > Issues: Read & Write
   - Repository > Metadata: Read-only (forced)
   - Repository > Checks: Read & Write
   - Organization > Members: Read-only

4. Subscribe to webhook events:
   - `pull_request`
   - `issue_comment`
   - `pull_request_review`
   - `pull_request_review_comment`

5. Set **"Where can this GitHub App be installed?"** to **"Only on this account"**.

6. Click **Create GitHub App**.

7. On the next page, note the **App ID** (a 6-7 digit number).
   ```bash
   export GITHUB_APP_ID=<the number shown>
   ```

8. Scroll down and click **Generate a private key**. A `.pem` file downloads automatically. Move it to a known location:
   ```bash
   mv ~/Downloads/tide-council-bot.*.pem ~/tide-council-bot.pem
   ```

### Step M3: Install the GitHub App on Tide Repos

1. On the App's settings page, click **Install App** in the left sidebar.
2. Click **Install** next to the `sei-protocol` org.
3. Select **Only select repositories** and choose:
   - `sei-protocol/tide-proposals`
   - `sei-protocol/tide-deliverables`
   - `sei-protocol/tide-workspace-blockchain-dev`
   - `sei-protocol/tide-workspace-k8s-specialist`
   - `sei-protocol/tide-workspace-platform-eng`
4. Click **Install**.
5. Note the **Installation ID** from the URL: `https://github.com/settings/installations/{INSTALLATION_ID}`.
   ```bash
   export GITHUB_INSTALLATION_ID=<the number in the URL>
   ```

### Step M4: Fund the Deployer Wallet

1. Go to https://arctic-1.sei.io/faucet (or use Sei Discord `#testnet-faucet`).
2. Paste your `DEPLOYER_ADDRESS`.
3. Request enough SEI to cover deployment gas plus agent funding. Budget 30 SEI total.
4. Verify balance:
   ```bash
   cast balance "$DEPLOYER_ADDRESS" --rpc-url https://evm-rpc-arctic-1.sei-apis.com --ether
   # Should show at least 20.0
   ```

### Step M5: Configure Ingress for Tekton EventListener

After deploying the EventListener (Step S6), you need to expose it to receive GitHub webhooks. For MVP on EKS:

```bash
# Get the EventListener service external IP/hostname after deployment
kubectl get svc -n tekton-tide el-tide-github
# Note the EXTERNAL-IP (LoadBalancer hostname or IP)
# Export it:
export TEKTON_EVENTLISTENER_URL=https://<external-ip-or-hostname>
```

Then return to the GitHub App settings and update the **Webhook URL** to `$TEKTON_EVENTLISTENER_URL/github`.

---

## Section 3: Scripted Setup (In Order)

Run these steps in sequence. Each step depends on the previous one completing successfully.

---

### Step S1: Store GitHub App Credentials in AWS Secrets Manager

**Script:** `scripts/setup-github-app.sh`

**What it does:**
- Stores the App private key, App ID, Installation ID, and webhook secret in AWS Secrets Manager
- Creates per-agent copies of the private key at paths the CSI driver expects
- Verifies the setup by generating a test installation token

**Inputs required:**
- `GITHUB_APP_ID` — from Step M2
- `GITHUB_INSTALLATION_ID` — from Step M3
- `~/tide-council-bot.pem` — from Step M2
- `WEBHOOK_SECRET` — the hex string you generated in Step M2
- `AWS_REGION` — set in prerequisites

**Run:**
```bash
WEBHOOK_SECRET=$(openssl rand -hex 32)  # or reuse the one from Step M2

./scripts/setup-github-app.sh \
  --org sei-protocol \
  --app-id "$GITHUB_APP_ID" \
  --private-key-file ~/tide-council-bot.pem \
  --installation-id "$GITHUB_INSTALLATION_ID" \
  --webhook-secret "$WEBHOOK_SECRET" \
  --aws-region "$AWS_REGION" \
  --agents "blockchain-dev,k8s-specialist,platform-eng,coordinator,reviewer" \
  --repos "tide-proposals,tide-deliverables,tide-workspace-blockchain-dev,tide-workspace-k8s-specialist,tide-workspace-platform-eng"

# Save the webhook secret for the next step
echo "Webhook secret: $WEBHOOK_SECRET"
echo "Store this in 1Password — you will paste it into GitHub App settings in Step M5"
```

**Outputs produced:**
| AWS SM Path | Content |
|-------------|---------|
| `tide/github/app-id` | GitHub App ID |
| `tide/github/app-private-key` | PEM private key |
| `tide/github/installation-id` | Installation ID |
| `tide/github/webhook-secret` | HMAC webhook secret |
| `tide/agents/blockchain-dev/github-app-key` | Copy of private key |
| `tide/agents/k8s-specialist/github-app-key` | Copy of private key |
| `tide/agents/platform-eng/github-app-key` | Copy of private key |
| `tide/agents/coordinator/github-app-key` | Copy of private key |
| `tide/agents/reviewer/github-app-key` | Copy of private key |

**Verify:**
```bash
# Script exits 0 and prints "SUCCESS: Installation token generated"
# Also verify manually:
aws secretsmanager list-secrets --region "$AWS_REGION" \
  --filter Key=name,Values=tide/ \
  --query 'SecretList[].Name' --output table
# Should show 9 secrets (4 shared + 5 per-agent)
```

---

### Step S2: Create KMS Signing Keys for Each Agent

**Script:** `scripts/create-kms-keys.sh`

**What it does:**
- Creates one `ECC_SECG_P256K1` (secp256k1) KMS key per agent
- Creates human-readable aliases (`alias/tide-agent-{name}`)
- Writes key IDs and ARNs to `deployments/agent-kms-keys.json`

**Inputs required:**
- `AWS_REGION` — already set

**Run:**
```bash
mkdir -p deployments
./scripts/create-kms-keys.sh
# Takes about 30 seconds for 5 keys
```

**Outputs produced:**
- `deployments/agent-kms-keys.json` — JSON array with `agent`, `kms_key_id`, `kms_key_arn` per agent
- 5 KMS keys in your AWS account
- 5 KMS aliases: `alias/tide-agent-blockchain-dev`, etc.

**Verify:**
```bash
aws kms list-aliases --region "$AWS_REGION" \
  --query 'Aliases[?starts_with(AliasName, `alias/tide-agent`)].AliasName' \
  --output table
# Should show 5 aliases

cat deployments/agent-kms-keys.json | jq '.[].agent'
# Should print 5 agent names
```

---

### Step S3: Derive Ethereum Addresses from KMS Public Keys

**Script:** `scripts/derive-agent-addresses.sh`

**What it does:**
- Reads each KMS key ARN from `deployments/agent-kms-keys.json`
- Fetches the DER-encoded secp256k1 public key from KMS
- Derives the Ethereum address (keccak256 of the 64-byte uncompressed public key, last 20 bytes)
- Writes the results to `deployments/agent-wallets.json`

**Inputs required:**
- `deployments/agent-kms-keys.json` — from Step S2
- `AWS_REGION` — already set

**Run:**
```bash
./scripts/derive-agent-addresses.sh
```

**Outputs produced:**
- `deployments/agent-wallets.json` — JSON array with `agent`, `kms_key_id`, `kms_key_arn`, `address` per agent

**Verify:**
```bash
cat deployments/agent-wallets.json | jq -r '.[] | "\(.agent): \(.address)"'
# Should show 5 agents with valid 0x-prefixed Ethereum addresses (42 chars each)

# Spot-check: confirm address is derived from the KMS key
# (Get public key directly and re-derive manually if needed)
```

---

### Step S4: Fund Agent Wallets with Testnet SEI

**Script:** `scripts/fund-agent-wallets.sh`

**What it does:**
- Reads agent addresses from `deployments/agent-wallets.json`
- Sends 2 SEI to each agent wallet from the deployer wallet

**Inputs required:**
- `DEPLOYER_KEY` — your deployer private key
- `deployments/agent-wallets.json` — from Step S3
- Deployer wallet funded with at least 12 SEI (2 per agent + gas)

**Run:**
```bash
export DEPLOYER_KEY=0x...  # from Step M4 — never commit this

./scripts/fund-agent-wallets.sh
# Each transfer takes a few seconds. Total: ~30 seconds.
```

**Outputs produced:** Nothing on disk. 5 on-chain transfers.

**Verify:**
```bash
# Check each agent balance
cat deployments/agent-wallets.json | jq -r '.[].address' | while read addr; do
  echo -n "$addr: "
  cast balance "$addr" --rpc-url https://evm-rpc-arctic-1.sei-apis.com --ether
done
# Each should show ~2.0 ETH (SEI)
```

---

### Step S5: Deploy Contracts to Arctic-1

**Script:** `forge script script/DeployMVP.s.sol`

**What it does:**
- Deploys `MockIdentityRegistry` with the deployer as owner
- Mints identity NFTs (token IDs 1-5) to each agent's address
- Deploys `TideCouncil` implementation contract
- Deploys `TideCouncil` UUPS proxy and initializes it
- Prints all deployed addresses

**Inputs required:**
- `deployments/agent-wallets.json` — from Step S3 (agent addresses)
- `DEPLOYER_KEY` and `DEPLOYER_ADDRESS` — from Step M4
- `DEFAULT_TTL=7200` — 2-hour proposal TTL for fast testnet iteration
- `DEFAULT_QUORUM=3` — 3-of-5 agents must approve

**Run:**
```bash
cd contracts/  # if contracts are in a subdirectory

# Install dependencies (first time only)
forge install OpenZeppelin/openzeppelin-contracts
forge install OpenZeppelin/openzeppelin-contracts-upgradeable
forge install foundry-rs/forge-std

# Build and test
forge build
forge test -vvv

# Extract agent addresses from the JSON
export AGENT_BLOCKCHAIN_DEV=$(cat ../deployments/agent-wallets.json | jq -r '.[] | select(.agent=="blockchain-dev") | .address')
export AGENT_K8S_SPECIALIST=$(cat ../deployments/agent-wallets.json | jq -r '.[] | select(.agent=="k8s-specialist") | .address')
export AGENT_PLATFORM_ENG=$(cat ../deployments/agent-wallets.json | jq -r '.[] | select(.agent=="platform-eng") | .address')
export AGENT_COORDINATOR=$(cat ../deployments/agent-wallets.json | jq -r '.[] | select(.agent=="coordinator") | .address')
export AGENT_REVIEWER=$(cat ../deployments/agent-wallets.json | jq -r '.[] | select(.agent=="reviewer") | .address')

export DEPLOYER_ADDRESS=0x...
export DEPLOYER_KEY=0x...
export DEFAULT_TTL=7200
export DEFAULT_QUORUM=3

# Deploy
forge script script/DeployMVP.s.sol:DeployMVP \
  --rpc-url https://evm-rpc-arctic-1.sei-apis.com \
  --broadcast \
  --verify \
  --verifier blockscout \
  --verifier-url https://seitrace.com/arctic-1/api \
  --private-key "$DEPLOYER_KEY" \
  -vvvv

cd ..
```

**Outputs produced:**
- Console output showing deployed addresses
- Foundry broadcast file at `contracts/broadcast/DeployMVP.s.sol/713715/run-latest.json`

**Immediately after deployment, record addresses:**
```bash
# Parse from Foundry broadcast (adjust paths as needed)
COUNCIL_PROXY=$(cat contracts/broadcast/DeployMVP.s.sol/713715/run-latest.json \
  | jq -r '.transactions[] | select(.contractName=="ERC1967Proxy") | .contractAddress')
COUNCIL_IMPL=$(cat contracts/broadcast/DeployMVP.s.sol/713715/run-latest.json \
  | jq -r '.transactions[] | select(.contractName=="TideCouncil") | .contractAddress')
IDENTITY_REGISTRY=$(cat contracts/broadcast/DeployMVP.s.sol/713715/run-latest.json \
  | jq -r '.transactions[] | select(.contractName=="MockIdentityRegistry") | .contractAddress')

echo "COUNCIL_PROXY=$COUNCIL_PROXY"
echo "COUNCIL_IMPL=$COUNCIL_IMPL"
echo "IDENTITY_REGISTRY=$IDENTITY_REGISTRY"

# Write deployments/arctic-1.json
cat > deployments/arctic-1.json << EOF
{
  "network": "arctic-1",
  "chain_id": 713715,
  "deployed_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "deployer": "$DEPLOYER_ADDRESS",
  "contracts": {
    "MockIdentityRegistry": { "address": "$IDENTITY_REGISTRY" },
    "TideCouncil_Implementation": { "address": "$COUNCIL_IMPL" },
    "TideCouncil_Proxy": { "address": "$COUNCIL_PROXY" }
  },
  "eip712_domain": {
    "name": "TideCouncil",
    "version": "1",
    "chainId": 713715,
    "verifyingContract": "$COUNCIL_PROXY"
  }
}
EOF

echo "Wrote deployments/arctic-1.json"
```

**Run post-deployment verification:**
```bash
export COUNCIL_PROXY IDENTITY_REGISTRY DEPLOYER_ADDRESS
export AGENT_BLOCKCHAIN_DEV AGENT_K8S_SPECIALIST AGENT_PLATFORM_ENG AGENT_COORDINATOR AGENT_REVIEWER

cd contracts/
forge script script/VerifyMVP.s.sol:VerifyMVP \
  --rpc-url https://evm-rpc-arctic-1.sei-apis.com \
  -vvvv
cd ..
# Should print "=== All Checks Passed ==="
```

---

### Step S6: Deploy Kubernetes Resources

**What it does:**
- Creates namespaces (`tide-system`, `tide-agents`, `tekton-tide`)
- Creates ServiceAccounts with IRSA annotations
- Creates the chain indexer Deployment + RBAC
- Creates the Tekton EventListener + TriggerBindings + TriggerTemplates
- Creates NetworkPolicies for agent pod isolation
- Creates SecretProviderClasses for each agent

**Before running, create per-agent ConfigMaps** from the deployment output. This step populates token IDs and KMS key ARNs:

```bash
# Build the per-agent ConfigMap data from deployment artifacts
for i in 0 1 2 3 4; do
  AGENT=$(cat deployments/agent-wallets.json | jq -r ".[$i].agent")
  ADDRESS=$(cat deployments/agent-wallets.json | jq -r ".[$i].address")
  KMS_ARN=$(cat deployments/agent-wallets.json | jq -r ".[$i].kms_key_arn")
  TOKEN_ID=$((i + 1))

  kubectl create configmap "tide-agent-${AGENT}-config" \
    --namespace tide-agents \
    --from-literal=agent-token-id="$TOKEN_ID" \
    --from-literal=kms-key-arn="$KMS_ARN" \
    --from-literal=provider-address="$ADDRESS" \
    --dry-run=client -o yaml | kubectl apply -f -
done
```

**Create the shared platform ConfigMap:**

```bash
kubectl create configmap tide-platform-config \
  --namespace tide-system \
  --from-literal=SEI_RPC_URL="https://evm-rpc-arctic-1.sei-apis.com" \
  --from-literal=COUNCIL_ADDRESS="$COUNCIL_PROXY" \
  --from-literal=JOB_HOOK_ADDRESS="0x0000000000000000000000000000000000000000" \
  --from-literal=INDEXER_START_BLOCK="0" \
  --dry-run=client -o yaml | kubectl apply -f -
```

**Create the shared Tekton ConfigMap:**

```bash
GITHUB_APP_ID=$(aws secretsmanager get-secret-value \
  --secret-id tide/github/app-id --region "$AWS_REGION" \
  --query SecretString --output text)
GITHUB_INSTALLATION_ID=$(aws secretsmanager get-secret-value \
  --secret-id tide/github/installation-id --region "$AWS_REGION" \
  --query SecretString --output text)

kubectl create configmap tide-config \
  --namespace tekton-tide \
  --from-literal=github-app-id="$GITHUB_APP_ID" \
  --from-literal=github-installation-id="$GITHUB_INSTALLATION_ID" \
  --from-literal=sei-rpc-url="https://evm-rpc-arctic-1.sei-apis.com" \
  --from-literal=sei-chain-id="713715" \
  --from-literal=council-address="$COUNCIL_PROXY" \
  --from-literal=job-hook-address="0x0000000000000000000000000000000000000000" \
  --from-literal=upstream-repo="sei-protocol/tide-repo" \
  --from-literal=proposals-repo="sei-protocol/tide-proposals" \
  --from-literal=deliverables-repo="sei-protocol/tide-deliverables" \
  --from-literal=llm-model="claude-sonnet-4-20250514" \
  --from-literal=aws-region="$AWS_REGION" \
  --dry-run=client -o yaml | kubectl apply -f -
```

**Apply all K8s manifests:**

```bash
# Apply in dependency order
kubectl apply -f manifests/tekton/base/namespace.yaml
kubectl apply -f manifests/base/namespaces/        # tide-system, tide-agents namespaces
kubectl apply -f manifests/base/rbac/              # Agent ServiceAccounts, IRSA annotations
kubectl apply -f manifests/tekton/base/rbac/       # EventListener SA + roles
kubectl apply -f manifests/tekton/base/eventlistener/
kubectl apply -f manifests/tekton/base/triggers/
kubectl apply -f manifests/tekton/base/tasks/
kubectl apply -f manifests/base/network-policies/
kubectl apply -f manifests/base/secret-provider-classes/

# Deploy chain indexer
kubectl apply -f manifests/base/chain-indexer/

# Verify all pods are ready
kubectl get pods -n tekton-tide
kubectl get pods -n tide-system
kubectl get eventlistener -n tekton-tide
```

**Create the webhook secret K8s Secret for Tekton:**

```bash
WEBHOOK_SECRET=$(aws secretsmanager get-secret-value \
  --secret-id tide/github/webhook-secret --region "$AWS_REGION" \
  --query SecretString --output text)

kubectl create secret generic tide-webhook-secret \
  --namespace tekton-tide \
  --from-literal=webhook-secret="$WEBHOOK_SECRET" \
  --dry-run=client -o yaml | kubectl apply -f -
```

**After deployment, update the GitHub App webhook URL** (see Step M5 — you now have the EventListener external IP/hostname):

```bash
# Get the EventListener service URL
kubectl get svc el-tide-github -n tekton-tide
# Note EXTERNAL-IP

# Update in GitHub App settings: https://github.com/organizations/sei-protocol/settings/apps/tide-council-bot
# Webhook URL: https://<EXTERNAL-IP>/github
# Webhook secret: the value in tide/github/webhook-secret
```

---

### Step S7: Create IRSA IAM Roles

For each agent, create an IAM role that allows its K8s ServiceAccount to sign with its KMS key.

```bash
# Get OIDC provider URL for your EKS cluster
CLUSTER_NAME=your-eks-cluster-name
OIDC_PROVIDER=$(aws eks describe-cluster \
  --name "$CLUSTER_NAME" \
  --region "$AWS_REGION" \
  --query "cluster.identity.oidc.issuer" \
  --output text | sed 's|https://||')
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

echo "OIDC Provider: $OIDC_PROVIDER"
echo "AWS Account: $AWS_ACCOUNT_ID"

# Create one role per agent
for i in 0 1 2 3 4; do
  AGENT=$(cat deployments/agent-wallets.json | jq -r ".[$i].agent")
  KMS_ARN=$(cat deployments/agent-wallets.json | jq -r ".[$i].kms_key_arn")
  SA_NAME="tide-agent-${AGENT}"
  ROLE_NAME="tide-agent-${AGENT}-irsa"

  # Trust policy
  cat > /tmp/trust-policy.json << TRUSTEOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {
      "Federated": "arn:aws:iam::${AWS_ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER}"
    },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": {
        "${OIDC_PROVIDER}:sub": "system:serviceaccount:tide-agents:${SA_NAME}",
        "${OIDC_PROVIDER}:aud": "sts.amazonaws.com"
      }
    }
  }]
}
TRUSTEOF

  # Permissions policy
  cat > /tmp/permissions-policy.json << PERMEOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "KMSSigning",
      "Effect": "Allow",
      "Action": ["kms:Sign", "kms:GetPublicKey"],
      "Resource": "${KMS_ARN}"
    },
    {
      "Sid": "SecretsRead",
      "Effect": "Allow",
      "Action": ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"],
      "Resource": [
        "arn:aws:secretsmanager:${AWS_REGION}:${AWS_ACCOUNT_ID}:secret:tide/agents/${AGENT}/*",
        "arn:aws:secretsmanager:${AWS_REGION}:${AWS_ACCOUNT_ID}:secret:tide/config/*"
      ]
    }
  ]
}
PERMEOF

  # Create role
  aws iam create-role \
    --role-name "$ROLE_NAME" \
    --assume-role-policy-document file:///tmp/trust-policy.json \
    --region "$AWS_REGION" 2>/dev/null || echo "Role $ROLE_NAME already exists, updating..."

  # Attach inline policy
  aws iam put-role-policy \
    --role-name "$ROLE_NAME" \
    --policy-name "tide-agent-permissions" \
    --policy-document file:///tmp/permissions-policy.json \
    --region "$AWS_REGION"

  # Annotate the K8s ServiceAccount with the role ARN
  ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${ROLE_NAME}"
  kubectl annotate serviceaccount "$SA_NAME" \
    --namespace tide-agents \
    eks.amazonaws.com/role-arn="$ROLE_ARN" \
    --overwrite

  echo "Created IRSA role for $AGENT: $ROLE_ARN"
done
```

---

## Section 4: Verification Checklist

Work through this checklist top-to-bottom. Each item tells you what to run and what a passing result looks like.

### Infrastructure Health

```bash
# 1. Tekton components are running
kubectl get pods -n tekton-pipelines
# All pods: Running

kubectl get pods -n tekton-triggers
# All pods: Running

# 2. EventListener is ready
kubectl get eventlistener -n tekton-tide tide-github -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# Should print: True

kubectl get svc -n tekton-tide el-tide-github
# Should show EXTERNAL-IP (not <pending>)

# 3. Chain indexer is running
kubectl get pods -n tide-system -l app.kubernetes.io/name=tide-chain-indexer
# STATUS: Running

kubectl logs -n tide-system -l app.kubernetes.io/name=tide-chain-indexer --tail=20
# Should show: "no new blocks" or "processed N events"
# Should NOT show: connection errors to Sei RPC
```

### GitHub App and Secrets

```bash
# 4. All secrets exist in AWS SM
aws secretsmanager list-secrets --region "$AWS_REGION" \
  --filter Key=name,Values=tide/ \
  --query 'SecretList[].Name' --output table
# Should list 9 secrets minimum

# 5. Verify the App can generate a token (reuses setup script logic)
APP_ID=$(aws secretsmanager get-secret-value --secret-id tide/github/app-id --region "$AWS_REGION" --query SecretString --output text)
INSTALLATION_ID=$(aws secretsmanager get-secret-value --secret-id tide/github/installation-id --region "$AWS_REGION" --query SecretString --output text)
KEY=$(aws secretsmanager get-secret-value --secret-id tide/github/app-private-key --region "$AWS_REGION" --query SecretString --output text)

# Write key to temp file
echo "$KEY" > /tmp/test-app-key.pem
NOW=$(date +%s)
IAT=$((NOW - 60)); EXP=$((NOW + 600))
HEADER=$(echo -n '{"alg":"RS256","typ":"JWT"}' | base64 | tr '+/' '-_' | tr -d '=')
PAYLOAD=$(echo -n "{\"iat\":${IAT},\"exp\":${EXP},\"iss\":\"${APP_ID}\"}" | base64 | tr '+/' '-_' | tr -d '=')
SIG=$(echo -n "${HEADER}.${PAYLOAD}" | openssl dgst -sha256 -sign /tmp/test-app-key.pem | base64 | tr '+/' '-_' | tr -d '=')
JWT="${HEADER}.${PAYLOAD}.${SIG}"

TOKEN=$(curl -s -X POST \
  -H "Authorization: Bearer $JWT" \
  -H "Accept: application/vnd.github+json" \
  "https://api.github.com/app/installations/${INSTALLATION_ID}/access_tokens" | jq -r '.token')
rm /tmp/test-app-key.pem

[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] && echo "PASS: Installation token generated (${TOKEN:0:10}...)" || echo "FAIL: Could not generate token"
```

### On-Chain Contracts

```bash
# 6. Contract verification
COUNCIL_PROXY=$(cat deployments/arctic-1.json | jq -r '.contracts.TideCouncil_Proxy.address')
IDENTITY_REGISTRY=$(cat deployments/arctic-1.json | jq -r '.contracts.MockIdentityRegistry.address')

# Check identity registry has 5 tokens
SUPPLY=$(cast call "$IDENTITY_REGISTRY" "totalSupply()(uint256)" --rpc-url https://evm-rpc-arctic-1.sei-apis.com)
[ "$SUPPLY" = "5" ] && echo "PASS: 5 identity tokens minted" || echo "FAIL: Expected 5, got $SUPPLY"

# Check council is not paused
PAUSED=$(cast call "$COUNCIL_PROXY" "paused()(bool)" --rpc-url https://evm-rpc-arctic-1.sei-apis.com)
[ "$PAUSED" = "false" ] && echo "PASS: Council not paused" || echo "FAIL: Council is paused"

# Check EIP-712 domain (via proposalCount call to verify contract is responsive)
COUNT=$(cast call "$COUNCIL_PROXY" "proposalCount()(uint256)" --rpc-url https://evm-rpc-arctic-1.sei-apis.com)
[ "$COUNT" = "0" ] && echo "PASS: Proposal count is 0 (fresh deployment)" || echo "INFO: Proposal count is $COUNT"
```

### Full Loop Test

```bash
# 7. Trigger a test proposal and watch the pipeline
# First, verify the webhook is connected by checking GitHub App delivery history
# Go to: https://github.com/organizations/sei-protocol/settings/apps/tide-council-bot/advanced
# There should be no webhook deliveries yet (or only failed 404 ones from the placeholder URL)

# Open a test PR in tide-proposals with the tide/design label
cd /tmp
git clone https://github.com/sei-protocol/tide-proposals.git
cd tide-proposals
git checkout -b test/first-design-proposal

cat > proposals/test-proposal-001.md << 'PROPEOF'
# Test Proposal 001

## Status: Draft

## Summary

This is a test design proposal to verify the Tide agent council review pipeline.

## Design

Simple: do nothing. This proposal tests the plumbing.

## Acceptance Criteria

- At least one agent posts a review comment on this PR.
- The review is signed with the agent's identity (`[tide/reviewer]` header).
PROPEOF

git add proposals/test-proposal-001.md
git commit -m "feat: add test design proposal 001"
git push origin test/first-design-proposal

gh pr create \
  --repo sei-protocol/tide-proposals \
  --title "Test Proposal 001: Plumbing Verification" \
  --body "This PR tests the Tide agent review pipeline end-to-end." \
  --label "tide/design"

cd /tmp && rm -rf tide-proposals

# 8. Watch for TaskRun creation (should happen within ~30 seconds of PR creation)
watch kubectl get taskruns -n tide-agents

# Expected: You should see TaskRuns like:
#   tide-review-reviewer-xxxxx    Running
#   tide-review-blockchain-dev-xxxxx    Running
# etc.

# 9. Follow a TaskRun log
TASKRUN=$(kubectl get taskruns -n tide-agents \
  -l tide.sei.io/trigger=design-review \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
[ -n "$TASKRUN" ] && tkn taskrun logs "$TASKRUN" -n tide-agents -f || echo "No TaskRuns found yet"

# 10. Check that agent posted a review comment on the PR
PR_NUMBER=$(gh pr list --repo sei-protocol/tide-proposals --label "tide/design" \
  --json number --jq '.[0].number')
gh pr view "$PR_NUMBER" --repo sei-protocol/tide-proposals --comments | \
  grep -E "\[tide/" && echo "PASS: Agent review comment found" || echo "PENDING: No agent comments yet"
```

### Troubleshooting Quick Reference

| Symptom | Where to look | Likely cause |
|---------|--------------|-------------|
| No TaskRuns appear after PR creation | `kubectl logs -n tekton-triggers -l app=tekton-triggers-core` | Webhook not reaching EventListener; HMAC validation failing; CEL filter not matching |
| TaskRun fails immediately with exit 10 | `kubectl describe taskrun <name> -n tide-agents` | Missing env var — check ConfigMap names and keys |
| TaskRun fails with exit 11 | Pod events: `kubectl describe pod <name> -n tide-agents` | CSI driver not mounting secrets; SecretProviderClass references wrong AWS SM path |
| TaskRun fails with exit 20 | TaskRun logs | Git clone failed — check GitHub token generation; App installation permissions |
| Chain indexer logs show RPC errors | `kubectl logs -n tide-system -l app.kubernetes.io/name=tide-chain-indexer` | Wrong RPC URL in `tide-platform-config`; Sei testnet temporarily down |
| EventListener shows 500 from GitHub | GitHub App > Advanced > Recent Deliveries | HMAC secret mismatch between GitHub App settings and `tide-webhook-secret` K8s Secret |
| KMS signing fails (exit 50) | TaskRun logs | IRSA annotation missing or wrong on ServiceAccount; IAM role trust policy not matching SA namespace |
| `cast call` to contracts fails | Try with `--rpc-url https://evm-rpc-arctic-1.sei-apis.com` explicitly | Wrong COUNCIL_PROXY address; contract not deployed on this chain |

