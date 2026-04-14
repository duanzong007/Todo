package com.duanzong.todo.shell;

import android.Manifest;
import android.database.Cursor;
import android.provider.Telephony;

import com.getcapacitor.JSArray;
import com.getcapacitor.JSObject;
import com.getcapacitor.PermissionState;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import com.getcapacitor.annotation.Permission;
import com.getcapacitor.annotation.PermissionCallback;

import java.nio.charset.StandardCharsets;
import java.util.HashSet;
import java.util.Set;
import java.util.UUID;

@CapacitorPlugin(
    name = "SmsBridge",
    permissions = {
        @Permission(strings = { Manifest.permission.READ_SMS }, alias = "sms")
    }
)
public class SmsBridgePlugin extends Plugin {
    private static final String SMS_PERMISSION_ALIAS = "sms";
    private static final int MAX_RAW_MESSAGES = 400;
    private static final long THREE_MONTHS_MS = 1000L * 60L * 60L * 24L * 90L;

    @PluginMethod
    public void status(PluginCall call) {
        JSObject result = new JSObject();
        PermissionState permissionState = getPermissionState(SMS_PERMISSION_ALIAS);
        result.put("available", true);
        result.put("canReadSms", permissionState == PermissionState.GRANTED);
        result.put("permission", permissionState.toString());
        result.put("source", "android-native");
        call.resolve(result);
    }

    @PluginMethod
    public void readPickupMessages(PluginCall call) {
        if (getPermissionState(SMS_PERMISSION_ALIAS) != PermissionState.GRANTED) {
            requestPermissionForAlias(SMS_PERMISSION_ALIAS, call, "smsPermissionCallback");
            return;
        }
        resolveMessagesAsync(call);
    }

    @PermissionCallback
    public void smsPermissionCallback(PluginCall call) {
        if (getPermissionState(SMS_PERMISSION_ALIAS) != PermissionState.GRANTED) {
            JSObject result = new JSObject();
            result.put("ok", false);
            result.put("reason", "permission_denied");
            result.put("permission", getPermissionState(SMS_PERMISSION_ALIAS).toString());
            call.resolve(result);
            return;
        }

        resolveMessagesAsync(call);
    }

    private void resolveMessagesAsync(PluginCall call) {
        new Thread(() -> {
            JSObject result = new JSObject();
            long cutoff = System.currentTimeMillis() - THREE_MONTHS_MS;
            Set<String> seenIds = new HashSet<>();
            JSArray messages = new JSArray();
            String[] projection = new String[] {
                Telephony.Sms._ID,
                Telephony.Sms.ADDRESS,
                Telephony.Sms.BODY,
                Telephony.Sms.DATE,
                Telephony.Sms.TYPE
            };
            int count = 0;

            try (Cursor cursor = getContext()
                .getContentResolver()
                .query(
                    Telephony.Sms.CONTENT_URI,
                    projection,
                    Telephony.Sms.DATE + " >= ? AND " + Telephony.Sms.TYPE + " = ?",
                    new String[] { String.valueOf(cutoff), String.valueOf(Telephony.Sms.MESSAGE_TYPE_INBOX) },
                    Telephony.Sms.DATE + " DESC"
                )) {

                if (cursor != null) {
                    int idIndex = cursor.getColumnIndex(Telephony.Sms._ID);
                    int addressIndex = cursor.getColumnIndex(Telephony.Sms.ADDRESS);
                    int bodyIndex = cursor.getColumnIndex(Telephony.Sms.BODY);
                    int dateIndex = cursor.getColumnIndex(Telephony.Sms.DATE);

                    while (cursor.moveToNext() && count < MAX_RAW_MESSAGES) {
                        String address = addressIndex >= 0 ? cursor.getString(addressIndex) : "";
                        String body = bodyIndex >= 0 ? cursor.getString(bodyIndex) : "";
                        long date = dateIndex >= 0 ? cursor.getLong(dateIndex) : 0;
                        if (body == null || body.trim().isEmpty()) {
                            continue;
                        }

                        String stableId = buildStableMessageId(address, body, date);
                        if (seenIds.contains(stableId)) {
                            continue;
                        }
                        seenIds.add(stableId);

                        JSObject item = new JSObject();
                        item.put("id", stableId);
                        item.put("system_id", idIndex >= 0 ? cursor.getString(idIndex) : "");
                        item.put("address", address);
                        item.put("body", body);
                        item.put("date", date);
                        messages.put(item);
                        count++;
                    }
                }
            } catch (Exception exception) {
                result.put("ok", false);
                result.put("reason", "query_failed");
                result.put("message", exception.getMessage());
                resolveOnMainThread(call, result);
                return;
            }

            result.put("ok", true);
            result.put("count", count);
            result.put("messages", messages);
            result.put("permission", getPermissionState(SMS_PERMISSION_ALIAS).toString());
            resolveOnMainThread(call, result);
        }, "todo-sms-reader").start();
    }

    private void resolveOnMainThread(PluginCall call, JSObject payload) {
        if (getActivity() == null) {
            call.resolve(payload);
            return;
        }
        getActivity().runOnUiThread(() -> call.resolve(payload));
    }

    private String buildStableMessageId(String address, String body, long date) {
        String normalizedAddress = address == null ? "" : address.trim();
        String normalizedBody = body == null ? "" : body.trim();
        String payload = normalizedAddress + "\n" + date + "\n" + normalizedBody;
        return UUID.nameUUIDFromBytes(payload.getBytes(StandardCharsets.UTF_8)).toString();
    }
}
