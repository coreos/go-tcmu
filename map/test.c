#include <unistd.h>
#include <sys/types.h>
#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include <linux/target_core_user.h>

int main(int argc, char *argv[]) {
  char buf[128];
  memset(buf, 0x0, 128);
  struct tcmu_cmd_entry* cmd = (struct tcmu_cmd_entry*)buf;
  printf("%d\n", sizeof(struct tcmu_cmd_entry));
  cmd->hdr.len_op = 0x1;
  cmd->hdr.cmd_id = 0x2;
  cmd->hdr.kflags = 0x3;
  cmd->hdr.uflags = 0x4;
  cmd->req.iov_cnt = 0x5;
  cmd->req.iov_bidi_cnt = 0x6;
  cmd->req.iov_dif_cnt = 0x7;
  cmd->req.cdb_off = 0x8;
  cmd->req.__pad1 = 0xf;
  cmd->req.__pad2 = 0xf;
  cmd->req.iov[0].iov_base = (void*)0x23;
  cmd->req.iov[0].iov_len = 0x24;
  int i;
  for (i=0;i<128;i++) {
    printf("0x%02x ", buf[i]);
    if (i%16==15)
      printf("\n");
  }
  printf("sizeof iov %d\n", sizeof(cmd->req.iov[0]));
  printf("sizeof iov_base %d\n", sizeof(cmd->req.iov[0].iov_base));
  printf("sizeof iov_len %d\n", sizeof(cmd->req.iov[0].iov_len));
  memset(buf, 0x0, 128);
  cmd->rsp.scsi_status = 0x2;
  cmd->rsp.sense_buffer[0] = 0x6;
  cmd->rsp.sense_buffer[1] = 0x7;
  for (i=0;i<128;i++) {
    printf("0x%02x ", buf[i]);
    if (i%16==15)
      printf("\n");
  }
  memset(buf, 0x0, 128);
      printf("\n");
      printf("\n");
  struct tcmu_mailbox* mb = (struct tcmu_mailbox*)buf;
  mb->cmd_head = 0x07;
  mb->cmd_tail = 0x08;
  for (i=0;i<128;i++) {
    printf("0x%02x ", buf[i]);
    if (i%16==15)
      printf("\n");
  }
  return 0;
}
