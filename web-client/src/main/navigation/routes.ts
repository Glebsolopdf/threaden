export interface InitialRoute {
  activeVoiceId: string;
  groupId: string;
  groupVoiceId: string;
  inviteToken: string;
  temporaryCode: string;
  isDiscover: boolean;
  isSettings: boolean;
  isTemporary: boolean;
}

export function parseInitialRoute(pathname = location.pathname): InitialRoute {
  return {
    activeVoiceId: matchPath(pathname, /^\/group-voice-rooms\/([^/]+)/),
    groupId: matchPath(pathname, /^\/groups\/([^/]+)/),
    groupVoiceId: matchPath(pathname, /^\/groups\/([^/]+)\/voice\/?$/),
    inviteToken: matchPath(pathname, /^\/invite\/([^/]+)/),
    temporaryCode: matchPath(pathname, /^\/temporary\/([^/]+)/),
    isDiscover: pathname.startsWith("/discover"),
    isSettings: pathname.startsWith("/settings"),
    isTemporary: pathname.startsWith("/temporary"),
  };
}

function matchPath(pathname: string, pattern: RegExp): string {
  return pathname.match(pattern)?.[1] || "";
}
