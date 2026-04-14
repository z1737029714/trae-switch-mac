#import <Cocoa/Cocoa.h>

extern void TraeSwitchMenuBarHandleAction(int action, int providerIndex);

static NSInteger const TraeSwitchActionToggleProxy = 1;
static NSInteger const TraeSwitchActionShowWindow = 2;
static NSInteger const TraeSwitchActionQuit = 3;
static NSInteger const TraeSwitchActionSwitchProvider = 4;

@interface TraeSwitchStatusTarget : NSObject
@end

@implementation TraeSwitchStatusTarget

- (void)handleToggleProxy:(id)sender {
    TraeSwitchMenuBarHandleAction((int)TraeSwitchActionToggleProxy, -1);
}

- (void)handleShowWindow:(id)sender {
    TraeSwitchMenuBarHandleAction((int)TraeSwitchActionShowWindow, -1);
}

- (void)handleQuit:(id)sender {
    TraeSwitchMenuBarHandleAction((int)TraeSwitchActionQuit, -1);
}

- (void)handleSwitchProvider:(NSMenuItem *)sender {
    TraeSwitchMenuBarHandleAction((int)TraeSwitchActionSwitchProvider, (int)sender.tag);
}

@end

static NSStatusItem *traeStatusItem = nil;
static TraeSwitchStatusTarget *traeStatusTarget = nil;

static void TraeSwitchMenuBarEnsureMainThread(void) {
    if (traeStatusItem != nil) {
        return;
    }

    traeStatusTarget = [TraeSwitchStatusTarget new];
    traeStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    if (traeStatusItem.button != nil) {
        traeStatusItem.button.title = @"TS";
        traeStatusItem.button.toolTip = @"Trae Switch";
    }
    traeStatusItem.menu = [[NSMenu alloc] initWithTitle:@"Trae Switch"];
}

void TraeSwitchMenuBarEnsure(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        TraeSwitchMenuBarEnsureMainThread();
    });
}

void TraeSwitchMenuBarRefresh(const char *stateJSON) {
    if (stateJSON == NULL) {
        return;
    }

    NSString *jsonString = [NSString stringWithUTF8String:stateJSON];
    dispatch_async(dispatch_get_main_queue(), ^{
        TraeSwitchMenuBarEnsureMainThread();

        NSData *jsonData = [jsonString dataUsingEncoding:NSUTF8StringEncoding];
        NSError *jsonError = nil;
        NSDictionary *state = [NSJSONSerialization JSONObjectWithData:jsonData options:0 error:&jsonError];
        if (jsonError != nil || ![state isKindOfClass:[NSDictionary class]]) {
            return;
        }

        BOOL proxyRunning = [state[@"proxyRunning"] boolValue];
        NSString *activeProviderName = state[@"activeProviderName"];
        NSNumber *activeProviderIndex = state[@"activeProviderIndex"];
        NSArray *providers = state[@"providers"];

        if (activeProviderName == nil || [activeProviderName length] == 0) {
            activeProviderName = @"未选择";
        }
        if (providers == nil || ![providers isKindOfClass:[NSArray class]]) {
            providers = @[];
        }
        if (activeProviderIndex == nil) {
            activeProviderIndex = @(-1);
        }

        NSMenu *menu = [[NSMenu alloc] initWithTitle:@"Trae Switch"];

        NSMenuItem *toggleItem = [[NSMenuItem alloc] initWithTitle:(proxyRunning ? @"停止代理" : @"启动代理")
                                                            action:@selector(handleToggleProxy:)
                                                     keyEquivalent:@""];
        toggleItem.target = traeStatusTarget;
        [menu addItem:toggleItem];
        [menu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *currentProviderItem = [[NSMenuItem alloc] initWithTitle:[NSString stringWithFormat:@"当前服务商：%@", activeProviderName]
                                                                     action:nil
                                                              keyEquivalent:@""];
        currentProviderItem.enabled = NO;
        [menu addItem:currentProviderItem];

        NSMenu *providerMenu = [[NSMenu alloc] initWithTitle:@"切换服务商"];
        if ([providers count] == 0) {
            NSMenuItem *emptyItem = [[NSMenuItem alloc] initWithTitle:@"暂无服务商" action:nil keyEquivalent:@""];
            emptyItem.enabled = NO;
            [providerMenu addItem:emptyItem];
        } else {
            for (NSDictionary *provider in providers) {
                NSString *name = provider[@"name"];
                NSNumber *index = provider[@"index"];
                if (name == nil || index == nil) {
                    continue;
                }

                NSMenuItem *providerItem = [[NSMenuItem alloc] initWithTitle:name
                                                                       action:@selector(handleSwitchProvider:)
                                                                keyEquivalent:@""];
                providerItem.target = traeStatusTarget;
                providerItem.tag = [index integerValue];
                providerItem.state = ([activeProviderIndex integerValue] == [index integerValue]) ? NSControlStateValueOn : NSControlStateValueOff;
                [providerMenu addItem:providerItem];
            }
        }

        NSMenuItem *providerRoot = [[NSMenuItem alloc] initWithTitle:@"切换服务商" action:nil keyEquivalent:@""];
        providerRoot.submenu = providerMenu;
        [menu addItem:providerRoot];
        [menu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *showItem = [[NSMenuItem alloc] initWithTitle:@"打开主窗口"
                                                          action:@selector(handleShowWindow:)
                                                   keyEquivalent:@""];
        showItem.target = traeStatusTarget;
        [menu addItem:showItem];

        NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"退出"
                                                          action:@selector(handleQuit:)
                                                   keyEquivalent:@""];
        quitItem.target = traeStatusTarget;
        [menu addItem:quitItem];

        traeStatusItem.menu = menu;
        if (traeStatusItem.button != nil) {
            traeStatusItem.button.title = @"TS";
            traeStatusItem.button.toolTip = proxyRunning ? @"Trae Switch（代理运行中）" : @"Trae Switch（代理已停止）";
        }
    });
}

void TraeSwitchMenuBarShowError(const char *title, const char *message) {
    NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"Trae Switch";
    NSString *nsMessage = message ? [NSString stringWithUTF8String:message] : @"发生未知错误";

    dispatch_async(dispatch_get_main_queue(), ^{
        NSAlert *alert = [[NSAlert alloc] init];
        alert.alertStyle = NSAlertStyleWarning;
        alert.messageText = nsTitle;
        alert.informativeText = nsMessage;
        [alert addButtonWithTitle:@"确定"];
        [alert runModal];
    });
}

void TraeSwitchMenuBarClose(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (traeStatusItem != nil) {
            [[NSStatusBar systemStatusBar] removeStatusItem:traeStatusItem];
            traeStatusItem = nil;
        }
        traeStatusTarget = nil;
    });
}
