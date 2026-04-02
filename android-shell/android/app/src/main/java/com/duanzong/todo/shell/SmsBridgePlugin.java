package com.duanzong.todo.shell;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

@CapacitorPlugin(name = "SmsBridge")
public class SmsBridgePlugin extends Plugin {
    @PluginMethod
    public void status(PluginCall call) {
        JSObject result = new JSObject();
        result.put("available", false);
        result.put("canReadSms", false);
        result.put("reason", "not_implemented");
        result.put("source", "android-placeholder");
        call.resolve(result);
    }

    @PluginMethod
    public void readPickupMessages(PluginCall call) {
        JSObject result = new JSObject();
        result.put("ok", false);
        result.put("reason", "not_implemented");
        result.put("messages", new JSObject());
        call.resolve(result);
    }
}
