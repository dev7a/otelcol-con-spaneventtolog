# Changelog

All notable changes to this project will be documented in this file.

## [0.3.0] - 2025-04-13

### Added
- Default configuration now includes `resource.attributes` in `LogAttributesFrom` to ensure resource attributes like `service.name` are included in log records.

### Fixed
- Updated tests to align with the new default configuration.

## [0.1.0] - Initial release

- Project initialized.
- Standalone OpenTelemetry Collector connector for span event to log conversion.
