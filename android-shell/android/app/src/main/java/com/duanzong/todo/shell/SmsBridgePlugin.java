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

@CapacitorPlugin(
    name = "SmsBridge",
    permissions = {
        @Permission(strings = { Manifest.permission.READ_SMS }, alias = "sms")
    }
)
public class SmsBridgePlugin extends Plugin {
    private static final String SMS_PERMISSION_ALIAS = "sms";
    private static final int MAX_MESSAGES = 100;
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
        resolveMessages(call);
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

        resolveMessages(call);
    }

    private void resolveMessages(PluginCall call) {
        JSObject result = new JSObject();
        JSArray messages = new JSArray();
        long cutoff = System.currentTimeMillis() - THREE_MONTHS_MS;
        String[] projection = new String[] {
            Telephony.Sms._ID,
            Telephony.Sms.ADDRESS,
            Telephony.Sms.BODY,
            Telephony.Sms.DATE
        };

        try (Cursor cursor = getContext()
            .getContentResolver()
            .query(
                Telephony.Sms.Inbox.CONTENT_URI,
                projection,
                Telephony.Sms.DATE + " >= ?",
                new String[] { String.valueOf(cutoff) },
                Telephony.Sms.DATE + " DESC"
            )) {

            if (cursor != null) {
                int idIndex = cursor.getColumnIndex(Telephony.Sms._ID);
                int addressIndex = cursor.getColumnIndex(Telephony.Sms.ADDRESS);
                int bodyIndex = cursor.getColumnIndex(Telephony.Sms.BODY);
                int dateIndex = cursor.getColumnIndex(Telephony.Sms.DATE);

                int count = 0;
                while (cursor.moveToNext() && count < MAX_MESSAGES) {
                    String body = bodyIndex >= 0 ? cursor.getString(bodyIndex) : "";
                    if (body == null || body.trim().isEmpty()) {
                        continue;
                    }

                    JSObject item = new JSObject();
                    item.put("id", idIndex >= 0 ? cursor.getString(idIndex) : String.valueOf(count));
                    item.put("address", addressIndex >= 0 ? cursor.getString(addressIndex) : "");
                    item.put("body", body);
                    item.put("date", dateIndex >= 0 ? cursor.getLong(dateIndex) : 0);
                    messages.put(item);
                    count++;
                }
            }
        } catch (Exception exception) {
            result.put("ok", false);
            result.put("reason", "query_failed");
            result.put("message", exception.getMessage());
            call.resolve(result);
            return;
        }

        result.put("ok", true);
        result.put("messages", messages);
        result.put("permission", getPermissionState(SMS_PERMISSION_ALIAS).toString());
        call.resolve(result);
    }
}
