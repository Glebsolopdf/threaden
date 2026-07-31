import { Routes } from '@angular/router';
import { authGuard, guestGuard } from './core/auth/auth.guard';

export const routes: Routes = [
  {
    path: 'login',
    canActivate: [guestGuard],
    loadComponent: () => import('./features/auth/login.component').then((m) => m.LoginComponent),
    title: 'Вход · threaden',
  },
  {
    path: 'register',
    canActivate: [guestGuard],
    loadComponent: () => import('./features/auth/register.component').then((m) => m.RegisterComponent),
    title: 'Регистрация · threaden',
  },
  {
    path: '',
    canActivate: [authGuard],
    loadComponent: () => import('./features/shell/shell.component').then((m) => m.ShellComponent),
    children: [
      { path: '', loadComponent: () => import('./features/home/home.component').then((m) => m.HomeComponent), title: 'threaden' },
      { path: 'discover', loadComponent: () => import('./features/discover/discover.component').then((m) => m.DiscoverComponent), title: 'Поиск групп · threaden' },
      { path: 'groups/:groupId', loadComponent: () => import('./features/groups/group.component').then((m) => m.GroupComponent), title: 'Группа · threaden' },
      { path: 'groups/:groupId/voice', redirectTo: 'groups/:groupId' },
      { path: 'invite/:inviteToken', loadComponent: () => import('./features/groups/group.component').then((m) => m.GroupComponent), title: 'Приглашение · threaden' },
      { path: 'temporary', loadComponent: () => import('./features/voice/voice.component').then((m) => m.VoiceComponent), title: 'Временная комната · threaden' },
      { path: 'temporary/:temporaryCode', loadComponent: () => import('./features/voice/voice.component').then((m) => m.VoiceComponent), title: 'Временная комната · threaden' },
      { path: 'group-voice-rooms/:voiceRoomId', loadComponent: () => import('./features/voice/voice.component').then((m) => m.VoiceComponent), title: 'Голосовая комната · threaden' },
      { path: 'settings', redirectTo: '' },
      { path: 'profile', redirectTo: '' },
    ],
  },
  { path: '**', redirectTo: '' },
];
