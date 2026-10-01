// @vitest-environment node
import { describe, it, expect, vi } from 'vitest';
import { ApiClient } from '../api/client';
import { createAuthStore } from './store';
import { EMPTY_USER } from '../api/schemas';

describe('password sessions', () => {
  it('rejects malformed sessions without persisting a credential', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({token:'',user:{id:''}}))));
    const api = new ApiClient('http://localhost');
    await expect(api.passwordLogin('alice','secret')).rejects.toThrow();
    vi.unstubAllGlobals();
  });
  it('keeps a temporary password session out of normal bootstrap', async () => {
    const onLogin = vi.fn();
    const storage = {getItem:()=>null,setItem:vi.fn(),removeItem:vi.fn()};
    const api = new ApiClient('http://localhost');
    vi.spyOn(api,'passwordLogin').mockResolvedValue({token:'temporary',user:{...EMPTY_USER,id:'u1',requires_password_change:true}});
    const store = createAuthStore({api,storage,onLogin});
    await store.getState().loginWithPassword('alice','temporary');
    expect(store.getState().status).toBe('password_change_required');
    expect(onLogin).not.toHaveBeenCalled();
  });
});

it('ignores a password login that completes after endpoint cleanup', async () => {
  let finish!: (response: Response) => void;
  vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>((resolve) => { finish = resolve; })));
  const api = new ApiClient('http://localhost');
  const pending = api.passwordLogin('alice', 'secret');
  api.invalidateSession();
  finish(new Response(JSON.stringify({token:'old-token',user:{id:'old-user'}})));
  await expect(pending).rejects.toThrow('Server session changed');
  vi.unstubAllGlobals();
});

it('omits cookies and CSRF in desktop password requests', async () => {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({token:'new-token',user:{id:'u1'}})));
  vi.stubGlobal('fetch',fetch);
  const api = new ApiClient('http://localhost',{identity:{platform:'desktop'}});
  await api.passwordLogin('alice','secret');
  expect(fetch.mock.calls[0]?.[1]).toMatchObject({credentials:'omit',redirect:'error'});
  expect(fetch.mock.calls[0]?.[1].headers['X-CSRF-Token']).toBeUndefined();
  vi.unstubAllGlobals();
});

it('completes setup on the same user and only then starts normal bootstrap', async () => {
  const onLogin = vi.fn();
  const storage = {getItem:()=>null,setItem:vi.fn(),removeItem:vi.fn()};
  const api = new ApiClient('http://localhost');
  vi.spyOn(api,'setupPassword').mockResolvedValue({token:'bound',user:{...EMPTY_USER,id:'original'}});
  const store = createAuthStore({api,storage,onLogin});
  store.getState().setUser({...EMPTY_USER,id:'original',requires_account_setup:true});
  expect(store.getState().status).toBe('account_setup_required');
  await store.getState().setupPassword('alice','secret','Alice');
  expect(store.getState().user?.id).toBe('original');
  expect(store.getState().status).toBe('authenticated');
  expect(onLogin).toHaveBeenCalledTimes(1);
});

it('keeps temporary sessions available after an incorrect current password', async () => {
  const expired = vi.fn();
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({error:'Incorrect password',code:'invalid_credentials'}),{status:401})));
  const api = new ApiClient('http://localhost',{onUnauthorized:expired});
  api.setToken('temporary');
  await expect(api.changePassword('wrong','new-password')).rejects.toThrow('Incorrect password');
  expect(expired).not.toHaveBeenCalled();
  vi.unstubAllGlobals();
});

it('never enables device fallback for password or contradictory server modes', async () => {
  const { configStore } = await import('../config');
  configStore.getState().setAuthConfig({allowSignup:true,authMode:'password',passwordAuthAvailable:true,deviceAuthAvailable:true});
  expect(configStore.getState().deviceAuthAvailable).toBe(false);
  expect(configStore.getState().authConfigInvalid).toBe(true);
  configStore.getState().setAuthConfig({allowSignup:true});
  expect(configStore.getState().passwordAuthAvailable).toBe(false);
  expect(configStore.getState().authConfigInvalid).toBe(false);
});

it('exposes the server retry interval without retrying a password automatically', async () => {
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({error:'Too many attempts',code:'rate_limited'}),{status:429,headers:{'Retry-After':'23'}}));
  vi.stubGlobal('fetch',fetch);
  const api = new ApiClient('http://localhost');
  await expect(api.passwordLogin('alice','secret')).rejects.toMatchObject({retryAfterSeconds:23});
  expect(fetch).toHaveBeenCalledOnce();
  vi.unstubAllGlobals();
});
