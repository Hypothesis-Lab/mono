#!/usr/bin/env python3
"""Local stateful Direct API simulator for deterministic fault injection."""

import argparse
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import socket
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

SERVICES = {
    "campaigns": ("Campaigns", "Campaigns"),
    "adgroups": ("AdGroups", "AdGroups"),
    "adimages": ("AdImages", "AdImages"),
    "ads": ("Ads", "Ads"),
    "keywords": ("Keywords", "Keywords"),
    "bidmodifiers": ("BidModifiers", "BidModifiers"),
    "retargetinglists": ("RetargetingLists", "RetargetingLists"),
    "audiencetargets": ("AudienceTargets", "AudienceTargets"),
}
ACTIONS = {"http_error", "yandex_error", "item_error", "disconnect_before", "disconnect_after", "delay_before", "delay_after"}


def validate_rules(rules):
    if not isinstance(rules, list):
        raise ValueError("rules must be an array")
    for rule in rules:
        if not isinstance(rule, dict) or rule.get("service") not in SERVICES or rule.get("method") not in {"get", "add", "delete", "suspend"}:
            raise ValueError("each rule requires a known service and method")
        if type(rule.get("occurrence")) is not int or rule["occurrence"] < 1:
            raise ValueError("rule occurrence must be a positive integer")
        if rule.get("action") not in ACTIONS:
            raise ValueError("unknown rule action")
        if rule["action"] == "item_error" and rule["method"] not in {"add", "delete", "suspend"}:
            raise ValueError("item_error requires add, delete or suspend")
        if rule["action"] in {"delay_before", "delay_after"} and (type(rule.get("delay_ms")) is not int or not 0 <= rule["delay_ms"] <= 300000):
            raise ValueError("delay_ms must be an integer from 0 to 300000")
        if rule["action"] == "http_error" and (type(rule.get("status")) is not int or not 400 <= rule["status"] <= 599):
            raise ValueError("http_error requires status 400..599")
        if rule["action"] in {"item_error", "yandex_error"} and (type(rule.get("error_code")) is not int or rule["error_code"] <= 0):
            raise ValueError("provider errors require a positive error_code")


class Store:
    def __init__(self, directory):
        self.directory = directory
        directory.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.path = directory / "state.json"
        self.log = directory / "requests.jsonl"
        self.lock = threading.RLock()
        self.state = json.loads(self.path.read_text()) if self.path.exists() else {
            "next_id": 100000, "objects": {key: {} for key in SERVICES}, "counts": {}, "rules": []}
        validate_rules(self.state["rules"])

    def save(self):
        temporary = self.path.with_suffix(".tmp")
        temporary.write_text(json.dumps(self.state, ensure_ascii=False, indent=2) + "\n")
        os.chmod(temporary, 0o600)
        temporary.replace(self.path)

    def event(self, data):
        with self.lock:
            with self.log.open("a", encoding="utf-8") as output:
                os.chmod(self.log, 0o600)
                output.write(json.dumps(data, ensure_ascii=False) + "\n")

    def select_rule(self, service, method):
        key = service + "." + method
        with self.lock:
            count = self.state["counts"].get(key, 0) + 1
            self.state["counts"][key] = count
            rule = next((r for r in self.state["rules"] if r["service"] == service and r["method"] == method and r["occurrence"] == count), None)
            self.save()
            return count, copy.deepcopy(rule)

    def invoke(self, service, method, params):
        with self.lock:
            if method == "get":
                return self.get(service, params)
            if method == "add":
                result = self.add(service, params)
            elif method == "delete":
                result = self.delete(service, params)
            elif method == "suspend":
                result = self.suspend(params)
            else:
                return error(55, "Unsupported method")
            self.save()
            return result

    def get(self, service, params):
        criteria = params.get("SelectionCriteria", {})
        objects = list(self.state["objects"][service].values())
        keys = {"Ids": "Id", "CampaignIds": "CampaignId", "AdGroupIds": "AdGroupId", "AdImageHashes": "AdImageHash", "Types": "Type"}
        for key, field in keys.items():
            if key in criteria:
                objects = [item for item in objects if item.get(field) in criteria[key]]
        if "Associated" in criteria:
            objects = [item for item in objects if self.image_associated(item.get("AdImageHash")) == criteria["Associated"]]
        page = params.get("Page", {})
        offset, limit = int(page.get("Offset", 0)), int(page.get("Limit", 1000))
        selected = copy.deepcopy(objects[offset:offset + limit])
        if service == "adimages":
            for item in selected:
                item["Associated"] = self.image_associated(item["AdImageHash"])
        result = {SERVICES[service][1]: selected}
        if offset + limit < len(objects):
            result["LimitedBy"] = offset + limit
        return {"result": result}

    def image_associated(self, image_hash):
        return "YES" if any(image_hash in ad.get("ResponsiveAd", {}).get("_hashes", []) for ad in self.state["objects"]["ads"].values()) else "NO"

    def add(self, service, params):
        items = params.get(SERVICES[service][0])
        if not isinstance(items, list) or not items:
            return error(8000, "Missing input array")
        results = []
        for item in items:
            if not isinstance(item, dict):
                results.append(item_error(8000, "Invalid object"))
                continue
            try:
                results.append(self.add_one(service, item))
            except (ValueError, KeyError, TypeError) as exc:
                results.append(item_error(4000, str(exc)))
        return {"result": {"AddResults": results}}

    def add_one(self, service, item):
        objects = self.state["objects"]
        if service == "adimages":
            data = base64.b64decode(item["ImageData"], validate=True)
            if not data or not item.get("Name"):
                raise ValueError("image data and name required")
            image_hash = hashlib.sha256(data).hexdigest()
            objects[service].setdefault(image_hash, {"AdImageHash": image_hash, "Name": item["Name"], "Type": item.get("Type", "REGULAR")})
            return {"AdImageHash": image_hash}
        if service == "adgroups" and str(item.get("CampaignId")) not in objects["campaigns"]:
            raise ValueError("campaign does not exist")
        if service in {"ads", "keywords", "bidmodifiers", "audiencetargets"} and str(item.get("AdGroupId")) not in objects["adgroups"]:
            raise ValueError("ad group does not exist")
        if service == "bidmodifiers":
            adjustments = item.get("DemographicsAdjustments")
            if not isinstance(adjustments, list) or not adjustments:
                raise ValueError("DemographicsAdjustments required")
            ids = []
            for adjustment in adjustments:
                identifier = self.new_id()
                objects[service][str(identifier)] = {"Id": identifier, "AdGroupId": item["AdGroupId"], "Type": "DEMOGRAPHICS_ADJUSTMENT", "Level": "AD_GROUP", "DemographicsAdjustment": {**adjustment, "Enabled": "YES"}}
                ids.append(identifier)
            return {"Ids": ids}
        if service == "ads":
            if "Type" in item:
                raise ValueError("unknown parameter Type")
            ad = item.get("ResponsiveAd")
            if not isinstance(ad, dict) or not ad.get("Titles") or not ad.get("Texts") or not ad.get("Href") or not ad.get("AdImageHashes"):
                raise ValueError("ResponsiveAd fields required")
            if any(image_hash not in objects["adimages"] for image_hash in ad["AdImageHashes"]):
                raise ValueError("image does not exist")
        if service == "audiencetargets" and str(item.get("RetargetingListId")) not in objects["retargetinglists"]:
            raise ValueError("retargeting list does not exist")
        identifier = self.new_id()
        record = copy.deepcopy(item)
        record["Id"] = identifier
        if service == "campaigns":
            record.update({"Type": "UNIFIED_CAMPAIGN" if "UnifiedCampaign" in item else "CPM_BANNER_CAMPAIGN", "State": "OFF", "Status": "DRAFT"})
        elif service == "adgroups":
            record["Type"] = "UNIFIED_AD_GROUP" if "CpmBannerUserProfileAdGroup" not in item else "CPM_BANNER_AD_GROUP"
        elif service == "ads":
            ad = item["ResponsiveAd"]
            record.update({"Type": "RESPONSIVE_AD", "Status": "DRAFT", "State": "OFF", "ResponsiveAd": {"Titles": [{"Title": value} for value in ad["Titles"]], "Texts": [{"Text": value} for value in ad["Texts"]], "Href": ad["Href"], "AdImages": {"Items": [{"ImageHash": value} for value in ad["AdImageHashes"]]}, "_hashes": ad["AdImageHashes"]}})
        objects[service][str(identifier)] = record
        return {"Id": identifier}

    def new_id(self):
        self.state["next_id"] += 1
        return self.state["next_id"]

    def delete(self, service, params):
        criteria = params.get("SelectionCriteria", {})
        identifiers = criteria.get("AdImageHashes" if service == "adimages" else "Ids", [])
        results = []
        objects = self.state["objects"]
        for identifier in identifiers:
            item = objects[service].get(str(identifier))
            if item is None:
                results.append(item_error(8800, "Object not found"))
                continue
            if service == "campaigns" and any(x.get("CampaignId") == identifier for x in objects["adgroups"].values()):
                results.append(item_error(8301, "Campaign contains groups"))
                continue
            if service == "adgroups" and any(x.get("AdGroupId") == identifier for kind in ("ads", "keywords", "bidmodifiers", "audiencetargets") for x in objects[kind].values()):
                results.append(item_error(8301, "Group contains dependent objects"))
                continue
            if service == "adimages" and self.image_associated(identifier) == "YES":
                results.append(item_error(8301, "Image is associated with an ad"))
                continue
            if service == "retargetinglists" and any(x.get("RetargetingListId") == identifier for x in objects["audiencetargets"].values()):
                results.append(item_error(8301, "Retargeting list has dependent targets"))
                continue
            del objects[service][str(identifier)]
            results.append({"AdImageHash" if service == "adimages" else "Id": identifier})
        return {"result": {"DeleteResults": results}}

    def suspend(self, params):
        results = []
        for identifier in params.get("SelectionCriteria", {}).get("Ids", []):
            campaign = self.state["objects"]["campaigns"].get(str(identifier))
            if campaign is None:
                results.append(item_error(8800, "Campaign not found"))
            elif campaign["Status"] == "DRAFT":
                results.append(item_error(8300, "Draft campaign cannot be suspended"))
            else:
                campaign["State"] = "SUSPENDED"
                results.append({"Id": identifier})
        return {"result": {"SuspendResults": results}}


def error(code, message):
    return {"error": {"error_code": code, "error_string": message, "error_detail": "mock", "request_id": "mock"}}


def item_error(code, message):
    return {"Errors": [{"Code": code, "Message": message, "Details": "mock"}]}


class Handler(BaseHTTPRequestHandler):
    server_version = "YandexMock/1"

    def log_message(self, fmt, *args):
        pass

    def reply(self, status, value):
        data = json.dumps(value, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def body(self):
        length = int(self.headers.get("Content-Length", "0"))
        if length > 20 * 1024 * 1024:
            raise ValueError("request too large")
        value = json.loads(self.rfile.read(length))
        if not isinstance(value, dict):
            raise ValueError("JSON object required")
        return value

    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/health":
            return self.reply(200, {"status": "ok"})
        if path == "/admin/state":
            with self.server.store.lock:
                return self.reply(200, copy.deepcopy(self.server.store.state))
        self.reply(404, {"error": "not found"})

    def do_POST(self):
        path = urlparse(self.path).path
        try:
            body = self.body()
            if path == "/admin/rules":
                rules = body.get("rules")
                validate_rules(rules)
                with self.server.store.lock:
                    self.server.store.state["rules"] = copy.deepcopy(rules)
                    self.server.store.state["counts"] = {}
                    self.server.store.save()
                return self.reply(200, {"rules": rules, "counts_reset": True})
            parts = path.strip("/").split("/")
            if len(parts) != 3 or parts[0] != "json" or parts[1] not in {"v5", "v501"} or parts[2] not in SERVICES:
                return self.reply(404, {"error": "unsupported endpoint"})
            return self.handle_api(parts[2], body)
        except (ValueError, KeyError, TypeError, json.JSONDecodeError) as exc:
            self.reply(400, {"error": str(exc)})
        except (BrokenPipeError, ConnectionResetError):
            pass

    def handle_api(self, service, body):
        method = body.get("method", "")
        params = body.get("params", {})
        count, rule = self.server.store.select_rule(service, method)
        action = rule.get("action") if rule else None
        record = {"time_ns": time.time_ns(), "service": service, "method": method, "occurrence": count, "action": action, "effect": False}
        try:
            if action == "delay_before":
                time.sleep(rule["delay_ms"] / 1000)
            if action == "disconnect_before":
                return self.disconnect()
            if action == "http_error":
                record["http_status"] = rule["status"]
                return self.reply(rule["status"], error(rule.get("error_code", 1000), rule.get("message", "injected HTTP error")))
            if action == "yandex_error":
                record["error_code"] = rule["error_code"]
                return self.reply(200, error(rule["error_code"], rule.get("message", "injected provider error")))
            if action == "item_error":
                key = {"add": "AddResults", "delete": "DeleteResults", "suspend": "SuspendResults"}[method]
                record["error_code"] = rule["error_code"]
                return self.reply(200, {"result": {key: [item_error(rule["error_code"], rule.get("message", "injected item error"))]}})
            result = self.server.store.invoke(service, method, params)
            record["effect"] = method != "get" and "result" in result and any(not x.get("Errors") for x in next(iter(result["result"].values()), []))
            if method != "get":
                items = next(iter(result.get("result", {}).values()), [])
                record["results"] = [{key: item[key] for key in ("Id", "Ids", "AdImageHash", "Errors") if key in item} for item in items]
            if action == "delay_after":
                time.sleep(rule["delay_ms"] / 1000)
            if action == "disconnect_after":
                return self.disconnect()
            self.reply(200, result)
        finally:
            self.server.store.event(record)

    def disconnect(self):
        self.close_connection = True
        try:
            self.connection.shutdown(socket.SHUT_RDWR)
        except OSError:
            pass
        self.connection.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--port", type=int, default=18081)
    parser.add_argument("--state-dir", type=Path, required=True)
    parser.add_argument("--rules", type=Path, help="JSON file containing {rules: [...]}; resets request counters")
    args = parser.parse_args()
    store = Store(args.state_dir)
    if args.rules:
        rules = json.loads(args.rules.read_text())["rules"]
        validate_rules(rules)
        store.state["rules"], store.state["counts"] = rules, {}
        store.save()
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.store = store
    print(f"Yandex mock listening on http://127.0.0.1:{args.port}/json/v5", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
