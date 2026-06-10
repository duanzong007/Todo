package com.duanzong.todo.shell;

import android.app.PendingIntent;
import android.appwidget.AppWidgetManager;
import android.appwidget.AppWidgetProvider;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.graphics.Color;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.view.View;
import android.widget.RemoteViews;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class TodoWidgetProvider extends AppWidgetProvider {
    static final String ACTION_REFRESH = "com.duanzong.todo.shell.widget.REFRESH";
    static final String ACTION_ITEM = "com.duanzong.todo.shell.widget.ITEM";
    static final String ITEM_ACTION_OPEN = "open";
    static final String ITEM_ACTION_COMPLETE = "complete";
    static final String EXTRA_ITEM_ACTION = "item_action";
    static final String EXTRA_TASK_ID = "task_id";
    static final String EXTRA_TASK_SHARED = "task_shared";
    static final String EXTRA_APP_PATH = "app_path";

    private static final int DEFAULT_WIDGET_WIDTH_DP = 250;
    private static final int DEFAULT_WIDGET_HEIGHT_DP = 140;
    private static final int TWO_COLUMN_MIN_WIDTH_DP = 330;
    private static final int HEADER_HEIGHT_DP = 32;
    private static final int FOOTER_HEIGHT_DP = 22;
    private static final int VERTICAL_PADDING_DP = 28;
    private static final int ROW_STEP_DP = 51;
    private static final int MAX_VISIBLE_ROWS = 14;
    private static final ExecutorService EXECUTOR = Executors.newSingleThreadExecutor();

    @Override
    public void onReceive(Context context, Intent intent) {
        String action = intent == null ? "" : intent.getAction();
        if (AppWidgetManager.ACTION_APPWIDGET_UPDATE.equals(action) || ACTION_REFRESH.equals(action)) {
            PendingResult pendingResult = goAsync();
            int[] widgetIds = widgetIdsFromIntent(context, intent);
            updateWidgetsAsync(context, widgetIds, true, pendingResult);
            return;
        }
        if (ACTION_ITEM.equals(action)) {
            handleItemAction(context, intent);
            return;
        }
        super.onReceive(context, intent);
    }

    @Override
    public void onUpdate(Context context, AppWidgetManager appWidgetManager, int[] appWidgetIds) {
        updateWidgetsAsync(context, appWidgetIds, true, null);
    }

    @Override
    public void onAppWidgetOptionsChanged(
        Context context,
        AppWidgetManager appWidgetManager,
        int appWidgetId,
        Bundle newOptions
    ) {
        TodoWidgetDataFetcher.WidgetSnapshot cached = TodoWidgetDataFetcher.loadCached(context);
        renderWidget(context.getApplicationContext(), appWidgetId, cached, statusFor(cached, "已缓存"));
    }

    static void updateAllWidgets(Context context, boolean forceRefresh) {
        int[] widgetIds = allWidgetIds(context);
        if (widgetIds.length == 0) {
            return;
        }
        updateWidgetsAsync(context, widgetIds, forceRefresh, null);
    }

    private static void updateWidgetsAsync(
        Context context,
        int[] widgetIds,
        boolean forceRefresh,
        PendingResult pendingResult
    ) {
        Context appContext = context.getApplicationContext();
        EXECUTOR.execute(() -> {
            try {
                if (widgetIds == null || widgetIds.length == 0) {
                    return;
                }

                TodoWidgetDataFetcher.WidgetSnapshot cached = TodoWidgetDataFetcher.loadCached(appContext);
                renderWidgets(appContext, widgetIds, cached, forceRefresh ? "同步中" : statusFor(cached, "已缓存"));

                if (!forceRefresh) {
                    return;
                }

                try {
                    TodoWidgetDataFetcher.WidgetSnapshot fresh = TodoWidgetDataFetcher.fetch(appContext);
                    renderWidgets(appContext, widgetIds, fresh, statusFor(fresh, "已更新"));
                } catch (TodoWidgetDataFetcher.AuthRequiredException exception) {
                    renderWidgets(appContext, widgetIds, cached, cached.tasks.isEmpty() ? "打开 App 登录" : "登录已过期");
                } catch (Exception exception) {
                    String status = cached.tasks.isEmpty() ? "同步失败" : "同步失败，显示缓存";
                    renderWidgets(appContext, widgetIds, cached, status);
                }
            } finally {
                if (pendingResult != null) {
                    pendingResult.finish();
                }
            }
        });
    }

    private void handleItemAction(Context context, Intent intent) {
        String itemAction = intent == null ? "" : intent.getStringExtra(EXTRA_ITEM_ACTION);
        if (ITEM_ACTION_OPEN.equals(itemAction)) {
            openApp(context.getApplicationContext(), intent.getStringExtra(EXTRA_APP_PATH));
            return;
        }
        if (!ITEM_ACTION_COMPLETE.equals(itemAction)) {
            return;
        }

        if (intent.getBooleanExtra(EXTRA_TASK_SHARED, false)) {
            String taskID = intent.getStringExtra(EXTRA_TASK_ID);
            openApp(context.getApplicationContext(), sharedCompletionPath(taskID));
            return;
        }

        PendingResult pendingResult = goAsync();
        handleCompleteAction(context, intent, pendingResult);
    }

    private static void handleCompleteAction(Context context, Intent intent, PendingResult pendingResult) {
        Context appContext = context.getApplicationContext();
        EXECUTOR.execute(() -> {
            try {
                String taskID = intent == null ? "" : intent.getStringExtra(EXTRA_TASK_ID);
                if (taskID == null || taskID.trim().isEmpty()) {
                    return;
                }

                TodoWidgetDataFetcher.WidgetSnapshot fresh = TodoWidgetDataFetcher.completeTask(appContext, taskID);
                int[] widgetIds = allWidgetIds(appContext);
                renderWidgets(appContext, widgetIds, fresh, statusFor(fresh, "已更新"));
            } catch (TodoWidgetDataFetcher.AuthRequiredException exception) {
                openApp(appContext, "/");
            } catch (Exception exception) {
                int[] widgetIds = allWidgetIds(appContext);
                TodoWidgetDataFetcher.WidgetSnapshot cached = TodoWidgetDataFetcher.loadCached(appContext);
                renderWidgets(appContext, widgetIds, cached, "确认失败");
            } finally {
                pendingResult.finish();
            }
        });
    }

    private static void renderWidgets(
        Context context,
        int[] widgetIds,
        TodoWidgetDataFetcher.WidgetSnapshot snapshot,
        String status
    ) {
        for (int widgetId : widgetIds) {
            renderWidget(context, widgetId, snapshot, status);
        }
    }

    private static void renderWidget(
        Context context,
        int widgetId,
        TodoWidgetDataFetcher.WidgetSnapshot snapshot,
        String status
    ) {
        AppWidgetManager manager = AppWidgetManager.getInstance(context);
        RemoteViews views = buildRemoteViews(context, widgetId, snapshot, status);
        manager.updateAppWidget(widgetId, views);
        manager.notifyAppWidgetViewDataChanged(widgetId, R.id.todo_widget_list);
    }

    private static RemoteViews buildRemoteViews(
        Context context,
        int widgetId,
        TodoWidgetDataFetcher.WidgetSnapshot snapshot,
        String status
    ) {
        RemoteViews views = new RemoteViews(context.getPackageName(), R.layout.todo_widget);
        int total = Math.max(snapshot.totalTaskCount, snapshot.tasks.size());
        views.setTextViewText(R.id.todo_widget_status, status);
        views.setTextViewText(R.id.todo_widget_more, total + " 个任务");
        views.setViewVisibility(R.id.todo_widget_more, snapshot.tasks.isEmpty() && total == 0 ? View.GONE : View.VISIBLE);
        views.setOnClickPendingIntent(R.id.todo_widget_root, openAppIntent(context, "/"));
        views.setOnClickPendingIntent(R.id.todo_widget_refresh, refreshIntent(context));

        Intent adapterIntent = new Intent(context, TodoWidgetRemoteViewsService.class);
        adapterIntent.putExtra(AppWidgetManager.EXTRA_APPWIDGET_ID, widgetId);
        adapterIntent.setData(Uri.parse(adapterIntent.toUri(Intent.URI_INTENT_SCHEME)));
        views.setRemoteAdapter(R.id.todo_widget_list, adapterIntent);
        views.setEmptyView(R.id.todo_widget_list, R.id.todo_widget_empty);
        views.setPendingIntentTemplate(R.id.todo_widget_list, itemIntentTemplate(context));

        if (snapshot.tasks.isEmpty()) {
            String message = snapshot.emptyText == null || snapshot.emptyText.trim().isEmpty()
                ? "今天没有焦点任务"
                : snapshot.emptyText;
            views.setTextViewText(R.id.todo_widget_empty_text, message);
            views.setTextViewText(R.id.todo_widget_empty_meta, snapshot.emptyMeta == null ? "" : snapshot.emptyMeta);
        }

        return views;
    }

    static WidgetLayout resolveLayout(Bundle options, int taskCount) {
        int widthDp = Math.max(
            optionValue(options, AppWidgetManager.OPTION_APPWIDGET_MIN_WIDTH, DEFAULT_WIDGET_WIDTH_DP),
            optionValue(options, AppWidgetManager.OPTION_APPWIDGET_MAX_WIDTH, DEFAULT_WIDGET_WIDTH_DP)
        );
        int heightDp = Math.max(
            optionValue(options, AppWidgetManager.OPTION_APPWIDGET_MIN_HEIGHT, DEFAULT_WIDGET_HEIGHT_DP),
            optionValue(options, AppWidgetManager.OPTION_APPWIDGET_MAX_HEIGHT, DEFAULT_WIDGET_HEIGHT_DP)
        );
        int availableRowsDp = heightDp - HEADER_HEIGHT_DP - FOOTER_HEIGHT_DP - VERTICAL_PADDING_DP;
        int fullyVisibleRows = Math.max(1, availableRowsDp / ROW_STEP_DP);
        int filledRows = Math.max(1, (availableRowsDp + ROW_STEP_DP - 1) / ROW_STEP_DP);
        int columns = widthDp >= TWO_COLUMN_MIN_WIDTH_DP && taskCount > fullyVisibleRows ? 2 : 1;

        WidgetLayout layout = new WidgetLayout();
        layout.columns = columns;
        layout.visibleRows = Math.min(MAX_VISIBLE_ROWS, filledRows);
        return layout;
    }

    static void bindSingleTask(RemoteViews views, TodoWidgetDataFetcher.WidgetTask task) {
        bindTaskViews(
            views,
            R.id.todo_widget_task_single_root,
            R.id.todo_widget_task_cell_root,
            R.id.todo_widget_task_kind,
            R.id.todo_widget_task_title,
            R.id.todo_widget_task_complete_target,
            R.id.todo_widget_task_complete,
            task
        );
    }

    static void bindLeftTask(RemoteViews views, TodoWidgetDataFetcher.WidgetTask task) {
        bindTaskViews(
            views,
            R.id.todo_widget_task_pair_root,
            R.id.todo_widget_task_left,
            R.id.todo_widget_task_left_kind,
            R.id.todo_widget_task_left_title,
            R.id.todo_widget_task_left_complete_target,
            R.id.todo_widget_task_left_complete,
            task
        );
    }

    static void bindRightTask(RemoteViews views, TodoWidgetDataFetcher.WidgetTask task) {
        bindTaskViews(
            views,
            R.id.todo_widget_task_pair_root,
            R.id.todo_widget_task_right,
            R.id.todo_widget_task_right_kind,
            R.id.todo_widget_task_right_title,
            R.id.todo_widget_task_right_complete_target,
            R.id.todo_widget_task_right_complete,
            task
        );
    }

    static void bindEmptySingleTask(RemoteViews views) {
        bindEmptyTaskViews(
            views,
            R.id.todo_widget_task_single_root,
            R.id.todo_widget_task_cell_root,
            R.id.todo_widget_task_kind,
            R.id.todo_widget_task_title,
            R.id.todo_widget_task_complete_target,
            R.id.todo_widget_task_complete
        );
    }

    static void bindEmptyLeftTask(RemoteViews views) {
        bindEmptyTaskViews(
            views,
            R.id.todo_widget_task_pair_root,
            R.id.todo_widget_task_left,
            R.id.todo_widget_task_left_kind,
            R.id.todo_widget_task_left_title,
            R.id.todo_widget_task_left_complete_target,
            R.id.todo_widget_task_left_complete
        );
    }

    static void bindEmptyRightTask(RemoteViews views) {
        bindEmptyTaskViews(
            views,
            R.id.todo_widget_task_pair_root,
            R.id.todo_widget_task_right,
            R.id.todo_widget_task_right_kind,
            R.id.todo_widget_task_right_title,
            R.id.todo_widget_task_right_complete_target,
            R.id.todo_widget_task_right_complete
        );
    }

    private static void bindTaskViews(
        RemoteViews views,
        int rowRootId,
        int cellRootId,
        int kindId,
        int titleId,
        int completeTargetId,
        int completeId,
        TodoWidgetDataFetcher.WidgetTask task
    ) {
        Intent openIntent = openFillInIntent("/");
        views.setInt(cellRootId, "setBackgroundResource", R.drawable.todo_widget_row_background);
        views.setViewVisibility(cellRootId, View.VISIBLE);
        views.setViewVisibility(kindId, View.VISIBLE);
        views.setViewVisibility(titleId, View.VISIBLE);
        views.setTextViewText(kindId, kindLabel(task));
        views.setTextViewText(titleId, task.title);
        views.setTextColor(kindId, kindTextColor(task));
        views.setInt(kindId, "setBackgroundResource", kindBackground(task));

        views.setOnClickFillInIntent(rowRootId, openIntent);
        views.setOnClickFillInIntent(cellRootId, openIntent);
        views.setOnClickFillInIntent(kindId, openIntent);
        views.setOnClickFillInIntent(titleId, openIntent);

        views.setViewVisibility(completeTargetId, task.canComplete ? View.VISIBLE : View.INVISIBLE);
        views.setViewVisibility(completeId, task.canComplete ? View.VISIBLE : View.INVISIBLE);
        views.setOnClickFillInIntent(completeTargetId, task.canComplete ? completeFillInIntent(task) : openIntent);
    }

    private static void bindEmptyTaskViews(
        RemoteViews views,
        int rowRootId,
        int cellRootId,
        int kindId,
        int titleId,
        int completeTargetId,
        int completeId
    ) {
        Intent openIntent = openFillInIntent("/");
        views.setInt(cellRootId, "setBackgroundColor", Color.TRANSPARENT);
        views.setViewVisibility(kindId, View.INVISIBLE);
        views.setTextViewText(kindId, "");
        views.setViewVisibility(titleId, View.VISIBLE);
        views.setTextViewText(titleId, "");
        views.setViewVisibility(completeTargetId, View.INVISIBLE);
        views.setViewVisibility(completeId, View.INVISIBLE);
        views.setOnClickFillInIntent(rowRootId, openIntent);
        views.setOnClickFillInIntent(cellRootId, openIntent);
    }

    private static String kindLabel(TodoWidgetDataFetcher.WidgetTask task) {
        String label = task.kindLabel == null ? "" : task.kindLabel.trim();
        return label.isEmpty() ? "任务" : label;
    }

    private static int kindTextColor(TodoWidgetDataFetcher.WidgetTask task) {
        String kind = normalizedKind(task);
        if ("todo".equals(kind)) {
            return Color.parseColor("#D135543B");
        }
        if ("schedule".equals(kind)) {
            return Color.parseColor("#CC31537A");
        }
        if ("ddl".equals(kind)) {
            return Color.parseColor("#D18A441F");
        }
        return Color.parseColor("#A9572F");
    }

    private static int kindBackground(TodoWidgetDataFetcher.WidgetTask task) {
        String kind = normalizedKind(task);
        if ("todo".equals(kind)) {
            return R.drawable.todo_widget_kind_todo_background;
        }
        if ("schedule".equals(kind)) {
            return R.drawable.todo_widget_kind_schedule_background;
        }
        if ("ddl".equals(kind)) {
            return R.drawable.todo_widget_kind_ddl_background;
        }
        return R.drawable.todo_widget_chip_background;
    }

    private static String normalizedKind(TodoWidgetDataFetcher.WidgetTask task) {
        String kindClass = task.kindClass == null ? "" : task.kindClass.trim().toLowerCase();
        if ("todo".equals(kindClass) || "schedule".equals(kindClass) || "ddl".equals(kindClass)) {
            return kindClass;
        }

        String label = task.kindLabel == null ? "" : task.kindLabel.trim().toLowerCase();
        if (label.contains("日程") || label.contains("schedule")) {
            return "schedule";
        }
        if (label.contains("ddl") || label.contains("截止")) {
            return "ddl";
        }
        return "todo";
    }

    private static Intent openFillInIntent(String path) {
        Intent intent = new Intent();
        intent.putExtra(EXTRA_ITEM_ACTION, ITEM_ACTION_OPEN);
        intent.putExtra(EXTRA_APP_PATH, path);
        return intent;
    }

    private static Intent completeFillInIntent(TodoWidgetDataFetcher.WidgetTask task) {
        Intent intent = new Intent();
        intent.putExtra(EXTRA_ITEM_ACTION, ITEM_ACTION_COMPLETE);
        intent.putExtra(EXTRA_TASK_ID, task.id);
        intent.putExtra(EXTRA_TASK_SHARED, task.hasCompletionUsers);
        return intent;
    }

    private static String sharedCompletionPath(String taskID) {
        Uri.Builder builder = new Uri.Builder().path("/");
        builder.appendQueryParameter("complete_task", taskID == null ? "" : taskID);
        builder.appendQueryParameter("widget_intent", String.valueOf(System.currentTimeMillis()));
        String path = builder.build().toString();
        return path.isEmpty() ? "/" : path;
    }

    private static String statusFor(TodoWidgetDataFetcher.WidgetSnapshot snapshot, String fallback) {
        String fetchedAt = TodoWidgetDataFetcher.formatFetchedAt(snapshot.fetchedAtMillis);
        if (fetchedAt.isEmpty()) {
            return fallback;
        }
        return fallback + " " + fetchedAt;
    }

    private static int optionValue(Bundle options, String key, int fallback) {
        if (options == null) {
            return fallback;
        }
        int value = options.getInt(key, fallback);
        return value > 0 ? value : fallback;
    }

    private static PendingIntent openAppIntent(Context context, String path) {
        Intent intent = new Intent(context, MainActivity.class);
        intent.setAction(Intent.ACTION_MAIN);
        intent.addCategory(Intent.CATEGORY_LAUNCHER);
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        intent.putExtra(MainActivity.EXTRA_APP_PATH, path);
        return PendingIntent.getActivity(context, stableRequestCode("open:" + path), intent, immutablePendingIntentFlags());
    }

    private static PendingIntent refreshIntent(Context context) {
        Intent intent = new Intent(context, TodoWidgetProvider.class);
        intent.setAction(ACTION_REFRESH);
        return PendingIntent.getBroadcast(context, 20, intent, immutablePendingIntentFlags());
    }

    private static PendingIntent itemIntentTemplate(Context context) {
        Intent intent = new Intent(context, TodoWidgetProvider.class);
        intent.setAction(ACTION_ITEM);
        return PendingIntent.getBroadcast(context, 30, intent, mutablePendingIntentFlags());
    }

    private static void openApp(Context context, String path) {
        Intent intent = new Intent(context, MainActivity.class);
        intent.setAction(Intent.ACTION_MAIN);
        intent.addCategory(Intent.CATEGORY_LAUNCHER);
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        intent.putExtra(MainActivity.EXTRA_APP_PATH, path == null || path.trim().isEmpty() ? "/" : path);
        context.startActivity(intent);
    }

    private static int immutablePendingIntentFlags() {
        int flags = PendingIntent.FLAG_UPDATE_CURRENT;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            flags |= PendingIntent.FLAG_IMMUTABLE;
        }
        return flags;
    }

    private static int mutablePendingIntentFlags() {
        int flags = PendingIntent.FLAG_UPDATE_CURRENT;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            flags |= PendingIntent.FLAG_MUTABLE;
        }
        return flags;
    }

    private static int stableRequestCode(String value) {
        return value == null ? 0 : value.hashCode();
    }

    private static int[] widgetIdsFromIntent(Context context, Intent intent) {
        int[] widgetIds = intent == null ? null : intent.getIntArrayExtra(AppWidgetManager.EXTRA_APPWIDGET_IDS);
        if (widgetIds == null || widgetIds.length == 0) {
            return allWidgetIds(context);
        }
        return widgetIds;
    }

    private static int[] allWidgetIds(Context context) {
        AppWidgetManager manager = AppWidgetManager.getInstance(context);
        return manager.getAppWidgetIds(new ComponentName(context, TodoWidgetProvider.class));
    }

    static final class WidgetLayout {
        int columns = 1;
        int visibleRows = 1;
    }
}
