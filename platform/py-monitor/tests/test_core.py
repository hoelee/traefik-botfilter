"""Unit tests for the extracted primitives (stdlib unittest, no deps)."""
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from monitor_core import (normalize_product_url, normalize_condition, parse_price,
                          strip_thumbnail_suffix, title_matches_keyword, iso_from_epoch, iso_now)


class UrlTests(unittest.TestCase):

    def test_strips_progressive_thumbnail(self):
        url = "https://media.example.com/abc.jpg_progressive_thumbnail"
        self.assertEqual("https://media.example.com/abc.jpg", strip_thumbnail_suffix(url))

    def test_strip_is_noop_without_suffix(self):
        url = "https://media.example.com/abc.jpg"
        self.assertEqual(url, strip_thumbnail_suffix(url))

    def test_strip_keeps_empty_input(self):
        self.assertEqual("", strip_thumbnail_suffix(""))

    def test_normalize_drops_query_and_fragment(self):
        a = "https://www.carousell.com.my/p/1234/?utm_source=x#top"
        b = "https://WWW.Carousell.com.my/p/1234/"
        self.assertEqual(normalize_product_url(a), normalize_product_url(b))
        self.assertEqual("https://www.carousell.com.my/p/1234", normalize_product_url(a))

    def test_normalize_handles_empty(self):
        self.assertEqual("", normalize_product_url(None))


class PriceTests(unittest.TestCase):

    def test_parses_thousands_separator(self):
        self.assertEqual(1250.5, parse_price("RM 1,250.50"))

    def test_parses_foreign_currency_prefix(self):
        self.assertEqual(12.0, parse_price("S$ 12"))

    def test_returns_none_for_text(self):
        self.assertIsNone(parse_price("Free"))
        self.assertIsNone(parse_price(""))
        self.assertIsNone(parse_price(None))

    def test_condition_aliases(self):
        self.assertEqual("Brand new", normalize_condition("Brand New"))
        self.assertEqual("Used", normalize_condition("used"))
        self.assertIsNone(normalize_condition("refurbished"))


class KeywordTests(unittest.TestCase):

    def test_case_insensitive_match(self):
        self.assertTrue(title_matches_keyword("iPhone 13 PROTECTOR", ["protector"]))

    def test_no_match(self):
        self.assertFalse(title_matches_keyword("iPhone 13", ["samsung"]))

    def test_empty_inputs(self):
        self.assertFalse(title_matches_keyword("", ["x"]))
        self.assertFalse(title_matches_keyword("x", []))


class TimeTests(unittest.TestCase):

    def test_iso_now_shape(self):
        self.assertRegex(iso_now(), r"^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$")

    def test_epoch_conversion_is_utc(self):
        self.assertEqual("2026-01-01 00:00:00", iso_from_epoch(1767225600))

    def test_falsy_epoch(self):
        self.assertIsNone(iso_from_epoch(0))


if __name__ == "__main__":
    unittest.main()
