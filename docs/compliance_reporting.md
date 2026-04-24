# Compliance Reporting System

This document describes the compliance and reporting capabilities implemented for the marketplace contract.

## Scope

The implementation covers:

- Transaction reporting
- Holder reporting
- Volume and taxable volume reporting
- Regulatory report generation and export formats
- Historical and real-time compliance monitoring
- Report scheduling
- Audit trail generation
- Regulatory compliance checks

## Contract Models

Primary models in `contracts/src/marketplace.rs`:

- `AssetComplianceSnapshot`
- `RegulatoryReport`
- `ReportExport`
- `ReportSchedule`
- `AuditTrailEntry`
- `TransferRestrictionConfig`
- `TransferApprovalRequest`

## Reporting Functions

The contract exposes reporting methods:

- `get_total_tokenized_value(asset_id, time_range)`
- `get_transaction_volume(asset_id, time_range)`
- `get_holder_count(asset_id, time_range)`
- `get_taxable_volume(asset_id, time_range)`
- `run_regulatory_checks()`
- `generate_regulatory_report(admin, asset_id, time_range, format)`
- `export_report(report_id)`
- `schedule_report(admin, asset_id, cadence_seconds, format)`
- `run_due_reports(admin)`
- `get_historical_compliance(asset_id, time_range)`
- `get_real_time_compliance(asset_id)`

## Report Formats

Report exports support:

- `ReportFormat::Json`
- `ReportFormat::Csv`

The contract returns the typed `ReportExport` enum so clients can serialize to final file formats off-chain.

## Audit Trail

Compliance-sensitive actions append immutable audit entries:

- asset registration/migration/deprecation
- listing creation/cancellation
- transfer approval request/review
- purchases and restricted transfers
- bulk whitelist operations

Each entry captures actor, action, asset, amount, and timestamp.

## Historical and Real-Time Monitoring

- Each purchase snapshots compliance metrics into `ComplianceHistory(asset_id)`.
- Historical queries filter snapshots by time range.
- Real-time queries provide the latest computed snapshot for an asset.

## Scheduling

Reports are scheduled per asset with cadence in seconds. The scheduler stores next run timestamp and can be triggered by automation (bot/relayer) using `run_due_reports`.

## Validation and Safety

- Time-range validation rejects invalid ranges.
- Admin-only operations are authorization-gated.
- Restricted transfer violations increment compliance-failure counters.
- Deprecated assets are blocked from new listings.

## Tests

Coverage includes:

- report generation and export
- scheduling and due-run execution
- transfer restrictions with approval workflow
- multi-asset isolation analytics

Run:

```bash
cd contracts
cargo test
```
