"use client";

import { useEffect, useRef, useState } from "react";
import type { NavigationAdapter } from "../navigation";

function signature(search: string): string {
  const params = new URLSearchParams(search);
  params.sort();
  return params.toString();
}

/** Keep input intent synchronous while the platform commits URL replacements. */
export function useTriageRoute(navigation: NavigationAdapter, path: string) {
  const routeSearch = navigation.searchParams.toString();
  const [intendedSearch, setIntendedSearch] = useState(routeSearch);
  const intended = useRef(routeSearch);
  const observed = useRef(routeSearch);
  const inFlight = useRef<string | null>(null);
  const replace = useRef(navigation.replace);
  replace.current = navigation.replace;

  const flush = () => {
    if (
      inFlight.current !== null ||
      signature(intended.current) === signature(observed.current)
    )
      return;
    inFlight.current = intended.current;
    replace.current(`${path}${intended.current ? `?${intended.current}` : ""}`);
  };

  useEffect(() => {
    if (routeSearch === observed.current) return;
    observed.current = routeSearch;
    if (
      inFlight.current !== null &&
      signature(routeSearch) === signature(inFlight.current)
    ) {
      inFlight.current = null;
      // Only one URL write is outstanding. An older acknowledgement can never
      // replace newer input; it merely releases the latest queued intention.
      if (signature(intended.current) !== signature(routeSearch)) {
        inFlight.current = intended.current;
        replace.current(
          `${path}${intended.current ? `?${intended.current}` : ""}`,
        );
      }
    } else {
      // Browser history / a link is an external navigation, not a stale form
      // callback. Adopt it so Back and shared filter URLs remain usable.
      inFlight.current = null;
      intended.current = routeSearch;
      setIntendedSearch(routeSearch);
    }
  }, [routeSearch, path]);

  const replaceParams = (params: URLSearchParams) => {
    intended.current = params.toString();
    setIntendedSearch(intended.current);
    flush();
  };
  const changeParams = (values: Record<string, string>) => {
    const next = new URLSearchParams(intended.current);
    for (const [key, value] of Object.entries(values)) {
      if (value) next.set(key, value);
      else next.delete(key);
    }
    replaceParams(next);
  };
  return {
    params: new URLSearchParams(intendedSearch),
    changeParams,
    replaceParams,
  };
}
