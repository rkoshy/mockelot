# Security Policy

## Supported Versions

We support the latest released version of Mockelot. Please update to the latest version to ensure you have the most recent security patches.

| Version | Supported          |
| ------- | ------------------ |
| Latest  | :white_check_mark: |
| < Latest| :x:                |

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

If you discover a security vulnerability in Mockelot, please report it privately.

### How to Report

Please email **renny@mockelot.com** (or the email listed in the GitHub profile of the maintainer) with the subject line `[Security] Vulnerability Report`.

Include the following details:
1.  **Description**: A clear description of the vulnerability.
2.  **Reproduction**: Steps to reproduce the issue (PoC code or configuration is helpful).
3.  **Impact**: What can an attacker achieve with this vulnerability?
4.  **Environment**: Operating system, Mockelot version, and relevant network configuration.

### Response Timeline

We are committed to addressing security issues promptly:
- **Acknowledgment**: Within 48 hours.
- **Assessment**: We will verify the issue and determine its severity.
- **Fix**: We will work on a patch and release it as soon as possible.

## Security Considerations for Users

Mockelot is a powerful development tool that acts as a **Man-in-the-Middle (MITM) proxy**. It creates self-signed certificates and intercepts traffic.

**WARNING:**
- **Do NOT run Mockelot on a public network** or expose its ports (`8080`, `1080`, `8443`) to the internet.
- **Do NOT install the Mockelot CA certificate on production machines** or devices you do not control.
- **Credentials**: SOCKS5 authentication credentials are currently stored in plain text in the configuration file. Do not commit your `config.json` or `.yaml` files if they contain sensitive secrets.

Mockelot is designed for **local development environments only**.
