#!/usr/bin/env python3
"""
OAuth2 Refresh Token Script

Executes an OAuth2 refresh token request to Keycloak and persists
the returned refresh token to a plain text file.

Usage:
    python3 oauth_token_refresh.py <REFRESH_TOKEN>
"""

import os
import sys
import urllib.request
import urllib.error
import json
import click

# Keycloak configuration
KEYCLOAK_BASE_URL = "https://example.com"
REALM = "example-realm"
TOKEN_ENDPOINT = (
    f"{KEYCLOAK_BASE_URL}/realms/{REALM}/protocol/openid-connect/token"
)
CLIENT_ID = "example-client-id"

# Token file path
TOKEN_FILE = os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    "refresh_token.txt",
)


def get_token_file_path():
    """Return the path to the token file."""
    return TOKEN_FILE


def read_token_from_file():
    """Read a refresh token from the token file if it exists."""
    if os.path.exists(TOKEN_FILE):
        with open(TOKEN_FILE, "r") as f:
            token = f.read().strip()
            if token:
                return token
    return None


def write_token_to_file(token):
    """Write a refresh token to the token file."""
    with open(TOKEN_FILE, "w") as f:
        f.write(token + "\n")


def refresh_access_token(refresh_token):
    """
    Execute the OAuth2 refresh token grant request.

    Args:
        refresh_token: The refresh token to use.

    Returns:
        The parsed JSON response from the token endpoint.

    Raises:
        urllib.error.HTTPError: If the request fails with an HTTP error.
        Exception: If the request fails for other reasons.
    """
    # Prepare the request body
    body = urllib.parse.urlencode({
        "grant_type": "refresh_token",
        "refresh_token": refresh_token,
        "client_id": CLIENT_ID,
    }).encode("utf-8")

    # Create the request
    req = urllib.request.Request(
        TOKEN_ENDPOINT,
        data=body,
        method="POST",
    )

    # Set headers
    req.add_header(
        "Content-Type",
        "application/x-www-form-urlencoded;charset=UTF-8",
    )
    req.add_header("Accept", "application/json")
    req.add_header("User-Agent", "monsoon-oauth-tool/1.0")

    click.echo(f"Sending refresh token request to: {TOKEN_ENDPOINT}", err=True)

    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            data = json.loads(response.read().decode("utf-8"))
            click.echo(f"Request successful (HTTP {response.status})", err=True)
            return data
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8", errors="replace")
        print(f"Request failed (HTTP {e.code}): {error_body}", file=sys.stderr)
        raise


@click.command()
@click.argument("refresh_token", required=False)
@click.option(
    "--token-file",
    default=TOKEN_FILE,
    help=f"Path to the token file (default: {TOKEN_FILE}).",
)
def main(token_file, refresh_token):
    """Refresh OAuth2 access token using a refresh token."""
    # Override token file if specified
    global TOKEN_FILE
    TOKEN_FILE = token_file

    # Use provided refresh token or read from file
    if not refresh_token:
        refresh_token = read_token_from_file()
        if not refresh_token:
            click.echo(
                "Error: No refresh token provided and no token found in file.",
                err=True,
            )
            sys.exit(1)

    # Perform the refresh token request
    try:
        response = refresh_access_token(refresh_token)
    except (urllib.error.HTTPError, Exception) as e:
        click.echo(f"Error: {e}", err=True)
        sys.exit(1)

    # Extract the new refresh token from the response
    new_refresh_token = response.get("refresh_token")
    if new_refresh_token:
        write_token_to_file(new_refresh_token)
        click.echo(f"New refresh token saved to: {TOKEN_FILE}", err=True)
    else:
        print(
            "Warning: No refresh token returned in the response.",
            file=sys.stderr,
        )

    # Print the new access token
    access_token = response.get("access_token", "N/A")
    expires_in = response.get("expires_in", "N/A")

    # print ttl
    click.echo(expires_in)
    click.echo(access_token)


if __name__ == "__main__":
    main()
