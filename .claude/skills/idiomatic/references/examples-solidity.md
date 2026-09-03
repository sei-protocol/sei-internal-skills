# Solidity idiom — worked examples

Loaded **on demand** by the method (step 3) when a worked before/after teaches faster than the rule alone — most useful for the §3 *divergences* and the *judgment-only* dimensions. Each example pairs with a dimension in `language-pack-solidity.md`; cite the same authority and §7 anchor. **Consult a pair for the pattern and cite it — do not paste the block wholesale into a review.**

These pairs are **original** (authored for this pack, not reproduced from any source — copyright-clean). For lint-anchored items, **"Anchor (observed)"** means the *bad* snippet was run through `slither`/`solhint`/`solc` and produced the quoted diagnostic, and the *good* snippet **resolves that anchor** (compiles and no longer trips the cited detector/rule). (A fully zero-warning Solidity contract additionally needs config choices that are partly idiom-divergent — see the §7 caveat on `func-visibility` constructors and the immutable-naming tool conflict — so the good fragments target their cited anchor, not every orthogonal lint.) For Slither, the line records the detector's **Impact+Confidence** (weight by it — an `Informational`/`Optimization` detector is a signal, not a blocker). For judgment-only items there is no detector — cite the prose Basis + SWC-ID and say so; never fabricate one.

How to read severity: a footgun marked **[SEC→defer]** is an exploitable class — flag the **idiomatic form** and cite the detector, then hand the exploit-depth verdict to `security-specialist` (pack §5/SEC1); do not present this lens as a security audit. A **judgment-only** item has no machine-checkable anchor. Everything else is **idiom/gas** or **style** — bundle style, never lead with it.

---

## Divergences — where modern (≥0.8) Solidity rejects older habits (judgment-only)

### §3 · SafeMath on ≥0.8 — delete it
Checked arithmetic reverts on overflow by default since 0.8.0; `SafeMath` is redundant.

```solidity
// bad
using SafeMath for uint256;
total = total.add(amount);
```
```solidity
// good — native checked arithmetic; unchecked{} only where wrap-around is proven safe
total = total + amount;
```
Basis: Security Considerations (overflow); SWC-101; §3. Anchor: **none — judgment-only** (no detector flags redundant SafeMath or an unsafe `unchecked` block).

### §3 / S2 · Push payments → pull-over-push
A single failing/malicious recipient must not be able to block everyone; let each address withdraw its own balance.

```solidity
// bad — one reverting recipient bricks the whole loop
for (uint256 i = 0; i < winners.length; i++) {
    winners[i].transfer(prize);
}
```
```solidity
// good — record entitlements; each winner pulls
mapping(address => uint256) public pending;
function claim() external {
    uint256 amt = pending[msg.sender];
    pending[msg.sender] = 0;          // effects before interaction (CEI)
    (bool ok,) = msg.sender.call{value: amt}("");
    require(ok, "transfer failed");
}
```
Basis: Common Patterns (withdrawal); SWC-113. Anchor: **none — judgment-only** (the withdrawal-pattern design choice isn't detector-modeled).

---

## Correctness (some `[SEC→defer]` — see each item's marker, not this header)

### S1 · State update after external call — reentrancy ([SEC→defer]) — `reentrancy-eth`
Update state *before* the external call (checks-effects-interactions), or a malicious fallback re-enters with the old balance.

```solidity
// bad — balance decremented AFTER the call; re-entrant withdraw drains the contract
function withdraw(uint256 amount) public {
    require(balances[msg.sender] >= amount, "insufficient");
    (bool ok,) = msg.sender.call{value: amount}("");
    balances[msg.sender] -= amount;
}
```
```solidity
// good — effects before interaction; check the result
function withdraw(uint256 amount) public {
    require(balances[msg.sender] >= amount, "insufficient");
    balances[msg.sender] -= amount;
    (bool ok,) = msg.sender.call{value: amount}("");
    require(ok, "transfer failed");
}
```
Basis: Security Considerations (CEI); SWC-107. Anchor (observed): `slither` → `reentrancy-eth` (**High+Medium**), `Reentrancy in Vault.withdraw(...)`. **[SEC→defer]:** flag the CEI fix and cite the detector; hand the exploit verdict to `security-specialist`.

### S2 · Unchecked low-level call — `unchecked-lowlevel` / `no-unused-vars`
A low-level `.call` returns `false` instead of reverting; ignoring it swallows the failure.

```solidity
// bad — ok captured, never checked
(bool ok,) = target.call(data);
```
```solidity
// good
(bool ok,) = target.call(data);
require(ok, "call failed");
```
Basis: Security Considerations; SWC-104. Anchor (observed): `slither` → `unchecked-lowlevel` (**Medium+Medium**); `solhint` → `Variable "ok" is unused` (`no-unused-vars`, recommended). A *bare* `target.call(data);` (no capture) also trips `solhint avoid-low-level-calls`.

### S3 · `tx.origin` auth ([SEC→defer]) — `tx-origin` / `avoid-tx-origin`
`tx.origin` is the original EOA — a phishing vector; authenticate with `msg.sender`.

```solidity
// bad
require(tx.origin == owner, "not owner");
```
```solidity
// good
require(msg.sender == owner, "not owner");
```
Basis: Security Considerations (tx.origin); SWC-115. Anchor (observed): `slither` → `tx-origin` (**Medium+Medium**); `solhint` → `avoid-tx-origin` (recommended). **[SEC→defer]** for the auth verdict.

### S6 · Unchecked ERC-20 transfer ([SEC→defer]) — `unchecked-transfer`
Non-standard tokens return `false` (or nothing) instead of reverting; check it / use `SafeERC20`.

```solidity
// bad — return ignored; a token that returns false silently "succeeds"
token.transfer(to, amount);
```
```solidity
// good — SafeERC20 reverts on failure
using SafeERC20 for IERC20;
token.safeTransfer(to, amount);
```
Basis: OpenZeppelin SafeERC20; SWC-104. Anchor (observed): `slither` → `unchecked-transfer` (**High+Medium**). **[SEC→defer]:** flag the checked-return / SafeERC20 fix and cite the detector; hand the exploit verdict to `security-specialist`. The *library choice* (OZ SafeERC20 specifically) is judgment-only — Slither flags the unchecked return, not which library you use.

### S3 · Address set with no zero-check — `missing-zero-check`
An `address` stored from input without a zero guard can brick the contract.

```solidity
// bad
function setOwner(address newOwner) external onlyOwner { owner = newOwner; }
```
```solidity
// good
function setOwner(address newOwner) external onlyOwner {
    require(newOwner != address(0), "zero address");
    owner = newOwner;
}
```
Basis: SWC (input validation). Anchor (observed): `slither` → `missing-zero-check` (**Low+Medium**; the detect-name; the wiki anchor reads "missing-zero-address-validation"). (Whether `setOwner` is gated at all is **[SEC→defer]** access-control — here shown already `onlyOwner`.)

---

## Idiom / gas

### S4 · `require`-string where a custom error fits — `gas-custom-errors`
Since 0.8.4, a named `error` + `revert` is cheaper and decodable.

```solidity
// bad
require(balances[msg.sender] >= amount, "insufficient balance");
```
```solidity
// good
error InsufficientBalance(uint256 have, uint256 want);
if (balances[msg.sender] < amount) revert InsufficientBalance(balances[msg.sender], amount);
```
Basis: OpenZeppelin v5 custom-error idiom; Solidity 0.8.4. Anchor (observed): `solhint` → `GC: Use Custom Errors instead of require statements` (`gas-custom-errors`, recommended/warn).

### S7 · Const-able / immutable-able storage — `constable-states` / `immutable-states`
A never-written var should be `constant`; a ctor-only var should be `immutable` — clearer and cheaper than a storage read.

```solidity
// bad
uint256 fee = 3;        // never reassigned
address admin;          // set only in the constructor
constructor() { admin = msg.sender; }
```
```solidity
// good
uint256 constant FEE = 3;
address immutable ADMIN;  // solhint immutable-vars-naming wants SNAKE_CASE; see §7 caveat
constructor() { ADMIN = msg.sender; }
```
Basis: Solidity language. Anchor (observed): `slither` → `constable-states` and `immutable-states` (both **Optimization**+High — gas signals, not defects).

### S9 · Floating pragma + missing SPDX — `solc-version` / `compiler-version` / solc
Pin the compiler and declare the license in deployable contracts.

```solidity
// bad
pragma solidity ^0.8.0;
contract C {}
```
```solidity
// good
// SPDX-License-Identifier: MIT
pragma solidity 0.8.35;
contract C {}
```
Basis: SWC-102/103. Anchor (observed): `slither` → `solc-version` (**Informational**+High); `solhint` → `compiler-version` (recommended, **error**-level); `solc` → `Warning: SPDX license identifier not provided`. (Informational impact — an idiom/reproducibility nit, not a blocking bug.)

### S3 · Implicit state visibility — `state-visibility`
State the visibility on every state variable.

```solidity
// bad
mapping(address => uint256) balances;
```
```solidity
// good
mapping(address => uint256) private balances;
```
Basis: SWC-108; Style Guide. Anchor (observed): `solhint` → `Explicitly mark visibility of state` (`state-visibility`, recommended/warn).

---
