#!/usr/bin/env python3
import sys
import unicorn as uc
import unicorn.arm_const as uca
import elftools.elf.elffile as ef

if len(sys.argv) < 2:
    print(f"usage: {sys.argv[0]} arm_elf_qt_binary...")
    exit(2)

errc = 0
for binary in sys.argv[1:]:
    try:
        found = 0
        with open(binary, "rb") as f:
            elf = ef.ELFFile(f)
            if elf.get_machine_arch() != "ARM":
                raise Exception(f"Only ARM binaries are supported, but {binary} is {elf.get_machine_arch()}")
            for sym in elf.get_section_by_name(".dynsym").iter_symbols():
                if len(sym.name)-3 >= 12 and sym.name.startswith(f"_Z{len(sym.name)-3-2}qInitResources_") and sym.name[-1] == "v":
                    found += 1
                    try:
                        qrc_name, qrc_init = sym.name[4+len("qInitResources_"):-1], sym["st_value"]
                        try:
                            # initialize unicorn
                            emu = uc.Uc(uc.UC_ARCH_ARM, uc.UC_MODE_THUMB if qrc_init&1 else uc.UC_MODE_ARM)

                            # load the elf
                            with open(binary, "rb") as f2:
                                for x in elf.iter_segments("PT_LOAD"):
                                    vaddr, offset, filesz, memsz, flags = x["p_vaddr"], x["p_offset"], x["p_filesz"], x["p_memsz"], x["p_flags"]
                                    f2.seek(offset)
                                    emu.mem_map(vaddr, memsz + (1024-memsz%1024), (uc.UC_PROT_EXEC if flags&0b001 else 0) + (uc.UC_PROT_WRITE if flags&0b010 else 0) + (uc.UC_PROT_READ if flags&0b100 else 0))
                                    emu.mem_write(vaddr, f2.read(filesz) + bytes([0]*(memsz - filesz)))

                            # initialize the stack
                            emu.mem_map(0x40000000 - 0x10000, 0x10000)
                            emu.reg_write(uca.UC_ARM_REG_SP, 0x40000000)

                            # r0, r1, r2, r3 should be overwritten later
                            emu.reg_write(uca.UC_ARM_REG_R0, 0xFFFFFFFF)
                            emu.reg_write(uca.UC_ARM_REG_R1, 0xFFFFFFFF)
                            emu.reg_write(uca.UC_ARM_REG_R2, 0xFFFFFFFF)
                            emu.reg_write(uca.UC_ARM_REG_R3, 0xFFFFFFFF)

                            # execute until a branch
                            def block_hook(emu, address, size, user_data):
                                if address != qrc_init&~1:
                                    raise Exception(f"branched")
                            emu.hook_add(uc.UC_HOOK_BLOCK, block_hook)
                            try:
                                emu.emu_start(qrc_init, qrc_init+128)
                            except Exception as err:
                                if str(err) != "branched":
                                    raise Exception(f"Failed to emulate") from err

                            # get the args to qRegisterResourceData
                            qrc_version = emu.reg_read(uca.UC_ARM_REG_R0)
                            qrc_tree = emu.reg_read(uca.UC_ARM_REG_R1)
                            qrc_names = emu.reg_read(uca.UC_ARM_REG_R2)
                            qrc_data = emu.reg_read(uca.UC_ARM_REG_R3)

                            # convert the offsets
                            if qrc_version == 0xFFFFFFFF or qrc_tree == 0xFFFFFFFF or qrc_names == 0xFFFFFFFF or qrc_data == 0xFFFFFFFF:
                                raise Exception("qInitResources didn't set arguments before branching")

                            def virt2file(addr):
                                for x in elf.iter_segments("PT_LOAD"):
                                    vaddr, offset, filesz, memsz, flags = x["p_vaddr"], x["p_offset"], x["p_filesz"], x["p_memsz"], x["p_flags"]
                                    if addr >= vaddr and addr < addr+filesz:
                                        return addr - vaddr + offset
                                raise Exception(f"Failed to find segment for address 0x{addr:X}")
                            qrc_tree, qrc_names, qrc_data = virt2file(qrc_tree), virt2file(qrc_names), virt2file(qrc_data)

                            print(f"{binary} {qrc_version} {qrc_tree:8d} {qrc_data:8d} {qrc_names:8d} # {qrc_name}")

                        except Exception as ex:
                            raise Exception(f"Failed to extract resources from qInitResources_{qrc_name}@0x{qrc_init:X}") from ex
                    except Exception as ex:
                        errc += 1
                        print(f"warning: {binary}: Failed to extract resources from {binary}: {ex}", file=sys.stderr)
        if found == 0:
            errc += 1
            print(f"error: {binary}: No resources found (you may need to look manually)", file=sys.stderr)
    except Exception as ex:
        errc += 1
        print(f"error: {binary}: {ex}", file=sys.stderr)
if errc != 0:
    exit(1)
