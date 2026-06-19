# otel-industrial-environments

Documentation and guidance for OpenTelemetry in industrial and operational technology (OT) environments.

The long-term goal is to develop content that may eventually be proposed to the OpenTelemetry documentation under *Platforms and Environments*.

## Documentation Format

This directory uses the Hugo content format and structure used by the OpenTelemetry documentation website.

This is intentional and allows content developed here to be easily migrated, reused, or contributed to the official OpenTelemetry documentation in the future.

When adding new content, contributors should follow the same Hugo structure and front matter conventions used by the OpenTelemetry documentation project.

## Current Structure

```text
otel-industrial-environments/
├── collector/
├── concepts/
├── getting-started/
├── protocols/
├── security/
├── semantic-conventions/
├── _index.md
└── adopters.md
```

### Directory Overview

| Directory              | Purpose                                                          |
| ---------------------- | ---------------------------------------------------------------- |
| `getting-started`      | Introductory guidance and onboarding content                     |
| `concepts`             | Core industrial observability concepts and terminology           |
| `protocols`            | Industrial protocols and integration guidance                    |
| `collector`            | Collector deployment patterns, configurations, and architectures |
| `semantic-conventions` | Industrial telemetry models and semantic conventions             |
| `security`             | Security, networking, and operational considerations             |
| `adopters.md`          | Industrial OpenTelemetry adoption examples and case studies      |

## Contributing

Ideas, use cases, examples, and documentation improvements are welcome.

Potential contribution areas include:

* Getting started guides and onboarding content
* Industrial observability concepts and terminology
* Industrial protocol documentation and integration guidance
* Collector deployment patterns, configurations, and reference architectures
* Industrial telemetry models and semantic convention proposals
* Security, networking, and operational best practices
* OpenTelemetry adoption stories, examples, and case studies

If you have an idea, use case, implementation experience, or documentation proposal that could help improve this content, please open an issue to start a discussion.

