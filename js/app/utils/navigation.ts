/**
 * Navigator structure mapping - defines which routes belong to which parent navigators
 */
const NAVIGATOR_STRUCTURE: Record<string, string[]> = {
  Home: ["StreamList", "Stream"],
  Settings: [
    "MainSettings",
    "AboutCategory",
    "AccountCategory",
    "StreamingCategory",
    "WebhooksSettings",
    "PrivacyCategory",
    "DanmuCategory",
    "AdvancedCategory",
    "LanguagesCategory",
    "DeveloperSettings",
    "KeyManagement",
  ],
};

/**
 * Finds the parent navigator for a given route name
 */
function findParentNavigator(routeName: string): string | null {
  for (const [parent, children] of Object.entries(NAVIGATOR_STRUCTURE)) {
    if (children.includes(routeName)) {
      return parent;
    }
  }
  return null;
}

/**
 * Navigates to a route, automatically handling nested navigators
 */
export function navigateToRoute(
  navigation: any,
  route: { name: string; params?: any },
) {
  const parent = findParentNavigator(route.name);

  if (parent) {
    // nested route - navigate to parent with screen param
    console.log(
      `navigateToRoute: ${route.name} is nested in ${parent}, navigating with screen param`,
    );
    (navigation.navigate as any)(parent, {
      screen: route.name,
      params: route.params,
    });
  } else {
    // top-level route - navigate directly
    console.log(
      `navigateToRoute: ${route.name} is top-level, navigating directly`,
    );
    (navigation.navigate as any)(route.name, route.params);
  }
}
