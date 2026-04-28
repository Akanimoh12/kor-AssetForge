# Security Policy

## Supported Versions

Currently, only the latest release of `kor-AssetForge` smart contracts is supported with security updates. 

| Version | Supported          |
| ------- | ------------------ |
| v0.1.0  | :white_check_mark: |

## Reporting a Vulnerability

We take the security of `kor-AssetForge` seriously. If you believe you have found a security vulnerability, please report it to us as described below.

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them by email to `security@kor-assetforge.example.com`. 

You should receive an acknowledgement within 48 hours, and depending on the severity of the issue, we will attempt to provide a timeline for the fix.

## Emergency Procedures

The `EmergencyControl` contract is integrated into the core operations (Trading, Minting, Transfers, Bridging). In the event of a critical vulnerability, the Admin can trigger a pause on affected scopes to prevent further exploitation while a fix is developed and deployed.

- **Trading Pause:** Disables listings and purchases on the marketplace.
- **Transfers Pause:** Disables direct token transfers between users.
- **Minting Pause:** Disables creation of new tokens or fractionalized assets.
- **Bridging Pause:** Disables cross-chain bridging operations.

In the case of the bridge being paused, an `emergency_withdraw` function is available to secure funds to a pre-configured cold wallet.

## Access Control

The contracts enforce strict role-based access control (`require_auth()`):
- **Admin**: Can upgrade contracts, configure parameters (fees, thresholds, emergency pauses), add/remove authorized bridge relayers.
- **Proposers/Voters**: Can propose and vote on governance actions.
- **Users**: Can interact with public endpoints (trade, transfer, stake). 

## Bug Bounty Program

We offer rewards for responsibly disclosed vulnerabilities that could lead to loss of funds, unauthorized state manipulation, or prolonged denial of service. The bounty amount is determined by the severity of the bug and the quality of the report.

## Recent Audits

All smart contracts have undergone internal review and third-party security audits. The primary areas addressed include:
- Reentrancy protection (CEI pattern and cross-contract checks)
- Integer overflow protection
- Input validation
- State consistency checks
