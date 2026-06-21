package com.duanzong.todo.shell;

import android.appwidget.AppWidgetManager;
import android.content.Context;
import android.content.Intent;
import android.os.Bundle;
import android.view.View;
import android.widget.RemoteViews;
import android.widget.RemoteViewsService;

public class TodoWidgetRemoteViewsService extends RemoteViewsService {
    @Override
    public RemoteViewsFactory onGetViewFactory(Intent intent) {
        int widgetId = intent == null
            ? AppWidgetManager.INVALID_APPWIDGET_ID
            : intent.getIntExtra(AppWidgetManager.EXTRA_APPWIDGET_ID, AppWidgetManager.INVALID_APPWIDGET_ID);
        return new TaskFactory(getApplicationContext(), widgetId);
    }

    private static final class TaskFactory implements RemoteViewsFactory {
        private final Context context;
        private final int widgetId;
        private TodoWidgetDataFetcher.WidgetSnapshot snapshot = TodoWidgetDataFetcher.WidgetSnapshot.empty();
        private TodoWidgetProvider.WidgetLayout layout = new TodoWidgetProvider.WidgetLayout();

        TaskFactory(Context context, int widgetId) {
            this.context = context;
            this.widgetId = widgetId;
        }

        @Override
        public void onCreate() {
            load();
        }

        @Override
        public void onDataSetChanged() {
            load();
        }

        @Override
        public void onDestroy() {
        }

        @Override
        public int getCount() {
            if (snapshot.tasks.isEmpty()) {
                return 0;
            }
            if (layout.columns == 2) {
                return Math.max(layout.visibleRows, (snapshot.tasks.size() + 1) / 2);
            }
            return Math.max(layout.visibleRows, snapshot.tasks.size());
        }

        @Override
        public RemoteViews getViewAt(int position) {
            if (layout.columns == 2) {
                return pairRow(position);
            }
            return singleRow(position);
        }

        @Override
        public RemoteViews getLoadingView() {
            return null;
        }

        @Override
        public int getViewTypeCount() {
            return 2;
        }

        @Override
        public long getItemId(int position) {
            return position;
        }

        @Override
        public boolean hasStableIds() {
            return false;
        }

        private void load() {
            snapshot = TodoWidgetDataFetcher.loadCached(context);
            Bundle options = AppWidgetManager.getInstance(context).getAppWidgetOptions(widgetId);
            layout = TodoWidgetProvider.resolveLayout(options, snapshot.tasks.size(), snapshot.widgetDualColumn);
        }

        private RemoteViews singleRow(int position) {
            RemoteViews row = new RemoteViews(context.getPackageName(), R.layout.todo_widget_task_single);
            if (position < 0 || position >= snapshot.tasks.size()) {
                TodoWidgetProvider.bindEmptySingleTask(row);
                return row;
            }

            TodoWidgetProvider.bindSingleTask(row, snapshot.tasks.get(position));
            return row;
        }

        private RemoteViews pairRow(int position) {
            RemoteViews row = new RemoteViews(context.getPackageName(), R.layout.todo_widget_task_pair);
            int leftIndex = position * 2;
            int rightIndex = leftIndex + 1;

            if (leftIndex < snapshot.tasks.size()) {
                row.setViewVisibility(R.id.todo_widget_task_left, View.VISIBLE);
                TodoWidgetProvider.bindLeftTask(row, snapshot.tasks.get(leftIndex));
            } else {
                TodoWidgetProvider.bindEmptyLeftTask(row);
            }

            if (rightIndex < snapshot.tasks.size()) {
                row.setViewVisibility(R.id.todo_widget_task_right, View.VISIBLE);
                TodoWidgetProvider.bindRightTask(row, snapshot.tasks.get(rightIndex));
            } else {
                TodoWidgetProvider.bindEmptyRightTask(row);
            }

            return row;
        }
    }
}
