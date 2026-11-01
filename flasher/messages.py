"""Safe, concise diagnostics for the installer window."""
from urllib.parse import quote, quote_plus

def error_message(error, *secrets):
    message = str(error) or type(error).__name__
    for secret in sorted((s for s in secrets if s), key=len, reverse=True):
        for value in (secret, quote(secret, safe=''), quote_plus(secret)):
            message = message.replace(value, '[redacted]')
    return message[:1800]
