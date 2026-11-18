"""Timestamp helpers - ISO 8601 in UTC, epoch in Asia/Kuala_Lumpur terms."""

import time
from datetime import datetime, timezone


def iso_now():
    # UTC: NocoDB parses naive datetimes as UTC.
    return time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime())


def iso_from_epoch(epoch):
    if not epoch:
        return None
    return time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(epoch))
