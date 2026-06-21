package com.duanzong.todo.shell;

import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.JSObject;
import com.getcapacitor.annotation.CapacitorPlugin;

@CapacitorPlugin(name = "AndroidUpdate")
public class AndroidUpdatePlugin extends Plugin {
    @PluginMethod
    public void status(PluginCall call) {
        call.resolve(AndroidUpdateManager.status(getActivity()));
    }

    @PluginMethod
    public void check(PluginCall call) {
        boolean manual = call.getBoolean("manual", true);
        AndroidUpdateManager.check(getActivity(), manual, call);
    }

    @PluginMethod
    public void refreshWidgets(PluginCall call) {
        TodoWidgetProvider.updateAllWidgets(getContext(), true);
        JSObject result = new JSObject();
        result.put("ok", true);
        call.resolve(result);
    }
}
