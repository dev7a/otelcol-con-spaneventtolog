# Changelog

All notable changes to this project will be documented in this file.

## [0.4.1] - 2025-06-08

### Changed
- Updated `golang.org/x/net` from v0.37.0 to v0.38.0
- Removed unused indirect dependencies to clean up dependency tree

## [0.4.0] - 2025-04-18

### Added
- Added `severity_attribute` configuration option to customize severity level mapping in log records
- Enhanced severity logic with improved attribute handling
- Expanded test coverage for severity attribute functionality

### Changed
- Updated connector logic to support configurable severity attributes
- Improved README documentation with severity attribute examples

### Fixed
- Refined severity mapping logic for better log record processing

## [0.3.1] - 2025-04-13

### Added
- Added `add_level` configuration option to copy severity text to a "level" attribute in log records, useful for log systems that expect a "level" field.

## [0.3.0] - 2025-04-13

### Added
- Default configuration now includes `resource.attributes` in `LogAttributesFrom` to ensure resource attributes like `service.name` are included in log records.

### Fixed
- Updated tests to align with the new default configuration.

## [0.2.0] - 2025-04-13

### Added
- Added proper Go module dependency management with go.mod and go.sum files
- Moved dependency files to repository root for better module structure

### Changed
- Reorganized project structure for improved dependency management
- Updated Makefile to work with new module structure

## [0.1.0] - Initial release

- Project initialized.
- Standalone OpenTelemetry Collector connector for span event to log conversion.
